// Package zkteco implements port.DeviceAdapter for ZKTeco devices via TCP/IP protocol.
//
// Dùng thư viện github.com/canhlinh/gozk (thuần Go, không cần cgo/DLL).
// Thư viện này hỗ trợ capture scan events theo thời gian thực.
// Toàn bộ dữ liệu SDK trả về được map sang domain model trước khi trả ra ngoài.
package zkteco

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/canhlinh/gozk"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
)

// Adapter implement port.DeviceAdapter cho thiết bị ZKTeco qua TCP/IP.
type Adapter struct {
	cfg      port.DeviceConfig
	zkClient *gozk.ZK
}

// New tạo instance mới cho ZKTeco adapter.
func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Connect(ctx context.Context, cfg port.DeviceConfig) error {
	a.cfg = cfg
	if cfg.IPAddress == "" {
		return fmt.Errorf("zkteco: ip_address is required")
	}

	port := cfg.Port
	if port == 0 {
		port = 4370 // cổng mặc định của ZKTeco
	}

	zk := gozk.NewZK(cfg.IPAddress,
		gozk.WithPort(port),
		gozk.WithTCP(true),
	)
	if err := zk.Connect(); err != nil {
		return fmt.Errorf("zkteco connect %s:%d: %w", cfg.IPAddress, port, err)
	}
	a.zkClient = zk
	return nil
}

func (a *Adapter) Disconnect(ctx context.Context) error {
	if a.zkClient == nil {
		return nil
	}
	return a.zkClient.Disconnect()
}

func (a *Adapter) CheckStatus(ctx context.Context) (port.DeviceStatus, error) {
	if a.zkClient == nil {
		return port.DeviceStatus{}, fmt.Errorf("zkteco: not connected")
	}
	fw, err := a.zkClient.GetFirmwareVersion()
	if err != nil {
		return port.DeviceStatus{}, fmt.Errorf("zkteco check status: %w", err)
	}
	props, _ := a.zkClient.GetProperties()
	userCount := 0
	logCount := 0
	if props != nil {
		userCount = props.TotalUsers
		logCount = props.TotalRecords
	}
	return port.DeviceStatus{
		Online:       true,
		FirmwareInfo: fw,
		UserCount:    userCount,
		LogCount:     logCount,
	}, nil
}

func (a *Adapter) SyncTime(ctx context.Context) error {
	if a.zkClient == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	return a.zkClient.SetTime(time.Now())
}

// GetEmployees: đọc danh sách nhân viên từ máy chấm công sử dụng COM SDK.
func (a *Adapter) GetEmployees(ctx context.Context) ([]entity.Employee, error) {
	if a.zkClient == nil {
		return nil, fmt.Errorf("zkteco: not connected")
	}
	emps, err := getEmployeesViaCOM(a.cfg.IPAddress, a.cfg.Port)
	if err != nil {
		return nil, fmt.Errorf("zkteco: GetEmployees failed: %w", err)
	}
	return emps, nil
}

func (a *Adapter) PushEmployee(ctx context.Context, emp entity.Employee) error {
	if a.zkClient == nil {
		return fmt.Errorf("zkteco: not connected")
	}

	// Thử đẩy thông tin nhân viên qua ZKTeco COM SDK (chỉ chạy trên Windows nếu có đăng ký DLL)
	if err := pushEmployeeViaCOM(a.cfg.IPAddress, a.cfg.Port, emp); err != nil {
		// Log lỗi chi tiết nhưng trả về thông báo lỗi thân thiện kèm chi tiết kỹ thuật cho người dùng
		return fmt.Errorf("zkteco: PushEmployee failed. Yêu cầu đăng ký SDK chính hãng (ZKemKeeper) trên Windows. Chi tiết lỗi COM: %w", err)
	}

	return nil
}

func (a *Adapter) PushFingerprint(ctx context.Context, employeeCode string, fp entity.EmployeeFingerprint) error {
	if a.zkClient == nil {
		return fmt.Errorf("zkteco: not connected")
	}

	if err := pushFingerprintViaCOM(a.cfg.IPAddress, a.cfg.Port, employeeCode, fp); err != nil {
		return fmt.Errorf("zkteco: PushFingerprint failed. Chi tiết lỗi COM: %w", err)
	}

	return nil
}

func (a *Adapter) GetFingerprint(ctx context.Context, employeeCode string, fingerIndex int) (*entity.EmployeeFingerprint, error) {
	if a.zkClient == nil {
		return nil, fmt.Errorf("zkteco: not connected")
	}

	fp, err := getFingerprintViaCOM(a.cfg.IPAddress, a.cfg.Port, employeeCode, fingerIndex)
	if err != nil {
		return nil, fmt.Errorf("zkteco: GetFingerprint failed. Chi tiết lỗi COM: %w", err)
	}

	return fp, nil
}

// pushEmployeeViaCOM thực hiện đẩy nhân viên xuống máy chấm công qua ZKTeco COM SDK (ZKemKeeper) qua go-ole.
func pushEmployeeViaCOM(ip string, port int, emp entity.Employee) error {
	// COM yêu cầu chạy trên cùng một OS Thread cố định
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitialize(0); err != nil {
		return fmt.Errorf("ole CoInitialize failed: %w", err)
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("zkemkeeper.ZKEM.1")
	if err != nil {
		return fmt.Errorf("không thể tạo đối tượng zkemkeeper.ZKEM.1 (hãy đăng ký file zkemkeeper.dll bằng quyền Admin: regsvr32 C:\\path\\to\\zkemkeeper.dll): %w", err)
	}
	defer unknown.Release()

	zkem, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("query IDispatch failed: %w", err)
	}
	defer zkem.Release()

	if port == 0 {
		port = 4370
	}

	res, err := oleutil.CallMethod(zkem, "Connect_Net", ip, int32(port))
	if err != nil {
		return fmt.Errorf("call Connect_Net failed: %w", err)
	}

	connected := false
	if res.VT == ole.VT_BOOL {
		connected = res.Value().(bool)
	} else {
		connected = res.Val == 1 || res.Value().(bool)
	}

	if !connected {
		errCodeVal, _ := oleutil.CallMethod(zkem, "GetLastError")
		var errCode int32
		if errCodeVal != nil {
			errCode = errCodeVal.Value().(int32)
		}
		return fmt.Errorf("kết nối máy chấm công qua COM SDK thất bại (Mã lỗi: %d)", errCode)
	}
	defer func() {
		_, _ = oleutil.CallMethod(zkem, "Disconnect")
	}()

	var dwMachineNumber int32 = 1

	// Gọi SSR_SetUserInfo để ghi người dùng xuống máy chấm công
	resPush, err := oleutil.CallMethod(zkem, "SSR_SetUserInfo",
		dwMachineNumber,
		emp.EmployeeCode,
		emp.FullName,
		"",           // Password rỗng
		int32(0),     // Quyền user thường (0)
		true,         // Kích hoạt tài khoản (enabled)
	)
	if err != nil {
		return fmt.Errorf("gọi SSR_SetUserInfo thất bại: %w", err)
	}

	success := false
	if resPush.VT == ole.VT_BOOL {
		success = resPush.Value().(bool)
	} else {
		success = resPush.Val == 1 || resPush.Value().(bool)
	}

	if !success {
		errCodeVal, _ := oleutil.CallMethod(zkem, "GetLastError")
		var errCode int32
		if errCodeVal != nil {
			errCode = errCodeVal.Value().(int32)
		}
		return fmt.Errorf("SSR_SetUserInfo trả về false (Mã lỗi: %d)", errCode)
	}

	// RefreshData để cập nhật hiển thị trên màn hình máy chấm công
	_, _ = oleutil.CallMethod(zkem, "RefreshData", dwMachineNumber)

	// Bắt đầu đăng ký vân tay ngón 0 (ngón đầu tiên) cho nhân viên ngay lập tức trên máy
	_, errStartEnroll := oleutil.CallMethod(zkem, "StartEnrollEx", emp.EmployeeCode, int32(0), int32(1))
	if errStartEnroll != nil {
		fmt.Printf("StartEnrollEx failed on device: %v\n", errStartEnroll)
	}

	return nil
}

// pushFingerprintViaCOM thực hiện đẩy template vân tay xuống máy chấm công qua ZKTeco COM SDK (ZKemKeeper) qua go-ole.
func pushFingerprintViaCOM(ip string, port int, employeeCode string, fp entity.EmployeeFingerprint) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitialize(0); err != nil {
		return fmt.Errorf("ole CoInitialize failed: %w", err)
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("zkemkeeper.ZKEM.1")
	if err != nil {
		return fmt.Errorf("không thể tạo đối tượng zkemkeeper.ZKEM.1: %w", err)
	}
	defer unknown.Release()

	zkem, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("query IDispatch failed: %w", err)
	}
	defer zkem.Release()

	if port == 0 {
		port = 4370
	}

	res, err := oleutil.CallMethod(zkem, "Connect_Net", ip, int32(port))
	if err != nil {
		return fmt.Errorf("call Connect_Net failed: %w", err)
	}

	connected := false
	if res.VT == ole.VT_BOOL {
		connected = res.Value().(bool)
	} else {
		connected = res.Val == 1 || res.Value().(bool)
	}

	if !connected {
		return fmt.Errorf("kết nối máy chấm công qua COM SDK thất bại")
	}
	defer func() {
		_, _ = oleutil.CallMethod(zkem, "Disconnect")
	}()

	var dwMachineNumber int32 = 1

	// SSR_SetUserTmpExStr ghi nhận template vân tay chuỗi Base64/Hex xuống máy
	resPush, err := oleutil.CallMethod(zkem, "SSR_SetUserTmpExStr",
		dwMachineNumber,
		employeeCode,
		int32(fp.FingerIndex),
		int32(1), // Flag = 1
		fp.TemplateData,
	)
	if err != nil {
		return fmt.Errorf("gọi SSR_SetUserTmpExStr thất bại: %w", err)
	}

	success := false
	if resPush.VT == ole.VT_BOOL {
		success = resPush.Value().(bool)
	} else {
		success = resPush.Val == 1 || resPush.Value().(bool)
	}

	if !success {
		return fmt.Errorf("SSR_SetUserTmpExStr trả về false")
	}

	_, _ = oleutil.CallMethod(zkem, "RefreshData", dwMachineNumber)
	return nil
}

// GetAttendanceLogs đọc toàn bộ attendance events đã lưu trên thiết bị và filter theo khoảng thời gian.
func (a *Adapter) GetAttendanceLogs(ctx context.Context, from, to time.Time) ([]entity.AttendanceLog, error) {
	if a.zkClient == nil {
		return nil, fmt.Errorf("zkteco: not connected")
	}

	events, err := a.zkClient.GetAllScannedEvents()
	if err != nil {
		return nil, fmt.Errorf("zkteco get attendance logs: %w", err)
	}

	var logs []entity.AttendanceLog
	for _, e := range events {
		if e.Error != nil {
			continue // bỏ qua event lỗi
		}
		t := e.Timestamp
		if t.Before(from) || t.After(to) {
			continue
		}
		rawBytes, _ := json.Marshal(map[string]interface{}{
			"device_id": e.DeviceID,
			"user_id":   e.UserID,
			"timestamp": e.Timestamp,
		})
		logs = append(logs, entity.AttendanceLog{
			EmployeeCode: fmt.Sprintf("%d", e.UserID),
			CheckTime:    t,
			CheckType:    entity.CheckTypeUnknown,
			VerifyMode:   entity.VerifyModeFingerprint,
			RawPayload:   rawBytes,
		})
	}
	return logs, nil
}

func (a *Adapter) ClearAttendanceLogs(ctx context.Context) error {
	if a.zkClient == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	return clearLogsViaCOM(a.cfg.IPAddress, a.cfg.Port)
}

func (a *Adapter) Reboot(ctx context.Context) error {
	if a.zkClient == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	return rebootViaCOM(a.cfg.IPAddress, a.cfg.Port)
}

func (a *Adapter) Reset(ctx context.Context) error {
	if a.zkClient == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	return resetViaCOM(a.cfg.IPAddress, a.cfg.Port)
}

func (a *Adapter) EnrollFingerprint(ctx context.Context, employeeCode string, fingerIndex int) error {
	if a.zkClient == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	return enrollFingerprintViaCOM(a.cfg.IPAddress, a.cfg.Port, employeeCode, fingerIndex)
}

// getFingerprintViaCOM thực hiện đọc template vân tay từ máy chấm công qua ZKTeco COM SDK (ZKemKeeper) qua go-ole.
func getFingerprintViaCOM(ip string, port int, employeeCode string, fingerIndex int) (*entity.EmployeeFingerprint, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitialize(0); err != nil {
		return nil, fmt.Errorf("ole CoInitialize failed: %w", err)
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("zkemkeeper.ZKEM.1")
	if err != nil {
		return nil, fmt.Errorf("không thể tạo đối tượng zkemkeeper.ZKEM.1: %w", err)
	}
	defer unknown.Release()

	zkem, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("query IDispatch failed: %w", err)
	}
	defer zkem.Release()

	if port == 0 {
		port = 4370
	}

	res, err := oleutil.CallMethod(zkem, "Connect_Net", ip, int32(port))
	if err != nil {
		return nil, fmt.Errorf("call Connect_Net failed: %w", err)
	}

	connected := false
	if res.VT == ole.VT_BOOL {
		connected = res.Value().(bool)
	} else {
		connected = res.Val == 1 || res.Value().(bool)
	}

	if !connected {
		return nil, fmt.Errorf("kết nối máy chấm công qua COM SDK thất bại")
	}
	defer func() {
		_, _ = oleutil.CallMethod(zkem, "Disconnect")
	}()

	var dwMachineNumber int32 = 1

	// Gọi GetUserTmpExStr
	vFlag := ole.NewVariant(ole.VT_I4, 0)
	vTmpData := ole.NewVariant(ole.VT_BSTR, 0)
	vTmpLength := ole.NewVariant(ole.VT_I4, 0)
	defer vFlag.Clear()
	defer vTmpData.Clear()
	defer vTmpLength.Clear()

	resGet, err := oleutil.CallMethod(zkem, "GetUserTmpExStr", dwMachineNumber, employeeCode, int32(fingerIndex), &vFlag, &vTmpData, &vTmpLength)
	if err != nil {
		return nil, fmt.Errorf("gọi GetUserTmpExStr thất bại: %w", err)
	}

	success := false
	if resGet.VT == ole.VT_BOOL {
		success = resGet.Value().(bool)
	} else {
		success = resGet.Val == 1 || resGet.Value().(bool)
	}

	if !success {
		// Thử fallback sang SSR_GetUserTmpStr nếu GetUserTmpExStr không được hỗ trợ
		vTmpDataFallback := ole.NewVariant(ole.VT_BSTR, 0)
		vTmpLengthFallback := ole.NewVariant(ole.VT_I4, 0)
		defer vTmpDataFallback.Clear()
		defer vTmpLengthFallback.Clear()

		resFallback, errFallback := oleutil.CallMethod(zkem, "SSR_GetUserTmpStr", dwMachineNumber, employeeCode, int32(fingerIndex), &vTmpDataFallback, &vTmpLengthFallback)
		if errFallback == nil {
			fallbackSuccess := false
			if resFallback.VT == ole.VT_BOOL {
				fallbackSuccess = resFallback.Value().(bool)
			} else {
				fallbackSuccess = resFallback.Val == 1 || resFallback.Value().(bool)
			}
			if fallbackSuccess {
				tmpData := vTmpDataFallback.Value().(string)
				tmpLength := vTmpLengthFallback.Value().(int32)
				return &entity.EmployeeFingerprint{
					FingerIndex:  fingerIndex,
					TemplateData: tmpData,
					TemplateSize: int(tmpLength),
					AlgoVersion:  "10.0",
				}, nil
			}
		}
		return nil, fmt.Errorf("không tìm thấy vân tay hoặc thiết bị không trả về template (Mã vân tay: %d)", fingerIndex)
	}

	tmpData := vTmpData.Value().(string)
	tmpLength := vTmpLength.Value().(int32)

	return &entity.EmployeeFingerprint{
		FingerIndex:  fingerIndex,
		TemplateData: tmpData,
		TemplateSize: int(tmpLength),
		AlgoVersion:  "10.0",
	}, nil
}

// rebootViaCOM thực hiện khởi động lại máy chấm công qua ZKTeco COM SDK.
func rebootViaCOM(ip string, port int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitialize(0); err != nil {
		return fmt.Errorf("ole CoInitialize failed: %w", err)
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("zkemkeeper.ZKEM.1")
	if err != nil {
		return fmt.Errorf("không thể tạo đối tượng zkemkeeper.ZKEM.1: %w", err)
	}
	defer unknown.Release()

	zkem, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("query IDispatch failed: %w", err)
	}
	defer zkem.Release()

	if port == 0 {
		port = 4370
	}

	res, err := oleutil.CallMethod(zkem, "Connect_Net", ip, int32(port))
	if err != nil {
		return fmt.Errorf("call Connect_Net failed: %w", err)
	}

	connected := false
	if res.VT == ole.VT_BOOL {
		connected = res.Value().(bool)
	} else {
		connected = res.Val == 1 || res.Value().(bool)
	}

	if !connected {
		return fmt.Errorf("kết nối máy chấm công qua COM SDK thất bại")
	}
	defer func() {
		_, _ = oleutil.CallMethod(zkem, "Disconnect")
	}()

	var dwMachineNumber int32 = 1
	resReboot, err := oleutil.CallMethod(zkem, "RebootDevice", dwMachineNumber)
	if err != nil {
		return fmt.Errorf("gọi RebootDevice thất bại: %w", err)
	}

	success := false
	if resReboot.VT == ole.VT_BOOL {
		success = resReboot.Value().(bool)
	} else {
		success = resReboot.Val == 1 || resReboot.Value().(bool)
	}
	if !success {
		return fmt.Errorf("RebootDevice trả về false")
	}

	return nil
}

// clearLogsViaCOM xóa toàn bộ log chấm công trên máy sử dụng ZKTeco COM SDK.
func clearLogsViaCOM(ip string, port int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitialize(0); err != nil {
		return fmt.Errorf("ole CoInitialize failed: %w", err)
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("zkemkeeper.ZKEM.1")
	if err != nil {
		return fmt.Errorf("không thể tạo đối tượng zkemkeeper.ZKEM.1: %w", err)
	}
	defer unknown.Release()

	zkem, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("query IDispatch failed: %w", err)
	}
	defer zkem.Release()

	if port == 0 {
		port = 4370
	}

	res, err := oleutil.CallMethod(zkem, "Connect_Net", ip, int32(port))
	if err != nil {
		return fmt.Errorf("call Connect_Net failed: %w", err)
	}

	connected := false
	if res.VT == ole.VT_BOOL {
		connected = res.Value().(bool)
	} else {
		connected = res.Val == 1 || res.Value().(bool)
	}

	if !connected {
		return fmt.Errorf("kết nối máy chấm công qua COM SDK thất bại")
	}
	defer func() {
		_, _ = oleutil.CallMethod(zkem, "Disconnect")
	}()

	var dwMachineNumber int32 = 1
	// Gọi ClearData(dwMachineNumber, 1) để xóa Attendance Logs (Flag 1)
	resClear, err := oleutil.CallMethod(zkem, "ClearData", dwMachineNumber, int32(1))
	if err != nil {
		return fmt.Errorf("gọi ClearData (Flag 1) thất bại: %w", err)
	}

	success := false
	if resClear.VT == ole.VT_BOOL {
		success = resClear.Value().(bool)
	} else {
		success = resClear.Val == 1 || resClear.Value().(bool)
	}
	if !success {
		return fmt.Errorf("ClearData (Flag 1) trả về false")
	}

	// Gọi RefreshData để cập nhật trạng thái hiển thị trên màn hình máy chấm công
	_, _ = oleutil.CallMethod(zkem, "RefreshData", dwMachineNumber)

	return nil
}

// resetViaCOM thực hiện reset toàn bộ máy (xóa user, vân tay, log, op record) qua ZKTeco COM SDK.
func resetViaCOM(ip string, port int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitialize(0); err != nil {
		return fmt.Errorf("ole CoInitialize failed: %w", err)
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("zkemkeeper.ZKEM.1")
	if err != nil {
		return fmt.Errorf("không thể tạo đối tượng zkemkeeper.ZKEM.1: %w", err)
	}
	defer unknown.Release()

	zkem, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("query IDispatch failed: %w", err)
	}
	defer zkem.Release()

	if port == 0 {
		port = 4370
	}

	res, err := oleutil.CallMethod(zkem, "Connect_Net", ip, int32(port))
	if err != nil {
		return fmt.Errorf("call Connect_Net failed: %w", err)
	}

	connected := false
	if res.VT == ole.VT_BOOL {
		connected = res.Value().(bool)
	} else {
		connected = res.Val == 1 || res.Value().(bool)
	}

	if !connected {
		return fmt.Errorf("kết nối máy chấm công qua COM SDK thất bại")
	}
	defer func() {
		_, _ = oleutil.CallMethod(zkem, "Disconnect")
	}()

	var dwMachineNumber int32 = 1

	// 1. Xóa logs (Flag 1)
	res1, err := oleutil.CallMethod(zkem, "ClearData", dwMachineNumber, int32(1))
	if err != nil {
		return fmt.Errorf("xóa log thất bại: %w", err)
	}

	// 2. Xóa users (Flag 5 - User Info và Fingerprints)
	res2, err := oleutil.CallMethod(zkem, "ClearData", dwMachineNumber, int32(5))
	if err != nil {
		return fmt.Errorf("xóa user thất bại: %w", err)
	}

	// 3. Xóa operation records (Flag 4)
	res3, err := oleutil.CallMethod(zkem, "ClearData", dwMachineNumber, int32(4))
	if err != nil {
		return fmt.Errorf("xóa op log thất bại: %w", err)
	}

	success := true
	for _, r := range []*ole.VARIANT{res1, res2, res3} {
		s := false
		if r.VT == ole.VT_BOOL {
			s = r.Value().(bool)
		} else {
			s = r.Val == 1 || r.Value().(bool)
		}
		if !s {
			success = false
		}
	}

	if !success {
		return fmt.Errorf("lệnh ClearData trả về false khi thực hiện reset")
	}

	// Refresh thiết bị
	_, _ = oleutil.CallMethod(zkem, "RefreshData", dwMachineNumber)

	return nil
}

// getEmployeesViaCOM đọc toàn bộ danh sách nhân viên từ máy chấm công sử dụng ZKTeco COM SDK.
func getEmployeesViaCOM(ip string, port int) ([]entity.Employee, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitialize(0); err != nil {
		return nil, fmt.Errorf("ole CoInitialize failed: %w", err)
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("zkemkeeper.ZKEM.1")
	if err != nil {
		return nil, fmt.Errorf("không thể tạo đối tượng zkemkeeper.ZKEM.1: %w", err)
	}
	defer unknown.Release()

	zkem, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("query IDispatch failed: %w", err)
	}
	defer zkem.Release()

	if port == 0 {
		port = 4370
	}

	res, err := oleutil.CallMethod(zkem, "Connect_Net", ip, int32(port))
	if err != nil {
		return nil, fmt.Errorf("call Connect_Net failed: %w", err)
	}

	connected := false
	if res.VT == ole.VT_BOOL {
		connected = res.Value().(bool)
	} else {
		connected = res.Val == 1 || res.Value().(bool)
	}

	if !connected {
		return nil, fmt.Errorf("kết nối máy chấm công qua COM SDK thất bại")
	}
	defer func() {
		_, _ = oleutil.CallMethod(zkem, "Disconnect")
	}()

	var dwMachineNumber int32 = 1

	// Đọc toàn bộ UserID vào bộ nhớ đệm
	resRead, err := oleutil.CallMethod(zkem, "ReadAllUserID", dwMachineNumber)
	if err != nil {
		return nil, fmt.Errorf("gọi ReadAllUserID thất bại: %w", err)
	}

	readSuccess := false
	if resRead.VT == ole.VT_BOOL {
		readSuccess = resRead.Value().(bool)
	} else {
		readSuccess = resRead.Val == 1 || resRead.Value().(bool)
	}
	if !readSuccess {
		return nil, fmt.Errorf("ReadAllUserID trả về false")
	}

	var (
		vEnrollNumber = ole.NewVariant(ole.VT_BSTR, 0)
		vName         = ole.NewVariant(ole.VT_BSTR, 0)
		vPassword     = ole.NewVariant(ole.VT_BSTR, 0)
		vPrivilege    = ole.NewVariant(ole.VT_I4, 0)
		vEnabled      = ole.NewVariant(ole.VT_BOOL, 0)
	)
	defer vEnrollNumber.Clear()
	defer vName.Clear()
	defer vPassword.Clear()
	defer vPrivilege.Clear()
	defer vEnabled.Clear()

	var employees []entity.Employee
	for {
		resLoop, err := oleutil.CallMethod(zkem, "SSR_GetAllUserInfo", dwMachineNumber, &vEnrollNumber, &vName, &vPassword, &vPrivilege, &vEnabled)
		if err != nil {
			break
		}

		success := false
		if resLoop.VT == ole.VT_BOOL {
			success = resLoop.Value().(bool)
		} else {
			success = resLoop.Val == 1 || resLoop.Value().(bool)
		}
		if !success {
			break
		}

		code := ""
		if vEnrollNumber.Value() != nil {
			code = vEnrollNumber.Value().(string)
		}
		name := ""
		if vName.Value() != nil {
			name = vName.Value().(string)
		}

		employees = append(employees, entity.Employee{
			EmployeeCode: code,
			FullName:     name,
			Status:       "active",
		})
	}

	return employees, nil
}

// enrollFingerprintViaCOM bắt đầu đăng ký vân tay từ xa cho nhân viên ngay lập tức qua ZKTeco COM SDK.
func enrollFingerprintViaCOM(ip string, port int, employeeCode string, fingerIndex int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitialize(0); err != nil {
		return fmt.Errorf("ole CoInitialize failed: %w", err)
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("zkemkeeper.ZKEM.1")
	if err != nil {
		return fmt.Errorf("không thể tạo đối tượng zkemkeeper.ZKEM.1: %w", err)
	}
	defer unknown.Release()

	zkem, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("query IDispatch failed: %w", err)
	}
	defer zkem.Release()

	if port == 0 {
		port = 4370
	}

	res, err := oleutil.CallMethod(zkem, "Connect_Net", ip, int32(port))
	if err != nil {
		return fmt.Errorf("call Connect_Net failed: %w", err)
	}

	connected := false
	if res.VT == ole.VT_BOOL {
		connected = res.Value().(bool)
	} else {
		connected = res.Val == 1 || res.Value().(bool)
	}

	if !connected {
		return fmt.Errorf("kết nối máy chấm công qua COM SDK thất bại")
	}
	defer func() {
		_, _ = oleutil.CallMethod(zkem, "Disconnect")
	}()

	var dwMachineNumber int32 = 1

	// Bắt đầu đăng ký vân tay ngón chỉ định (fingerIndex) cho nhân viên ngay lập tức trên máy
	// Flag = 1: Register fingerprint, 3: Register card, etc. For fingerprint, Flag is 1.
	resEnroll, errStartEnroll := oleutil.CallMethod(zkem, "StartEnrollEx", employeeCode, int32(fingerIndex), int32(1))
	if errStartEnroll != nil {
		return fmt.Errorf("StartEnrollEx failed on device: %w", errStartEnroll)
	}

	success := false
	if resEnroll.VT == ole.VT_BOOL {
		success = resEnroll.Value().(bool)
	} else {
		success = resEnroll.Val == 1 || resEnroll.Value().(bool)
	}

	if !success {
		errCodeVal, _ := oleutil.CallMethod(zkem, "GetLastError")
		var errCode int32
		if errCodeVal != nil {
			errCode = errCodeVal.Value().(int32)
		}
		return fmt.Errorf("StartEnrollEx returned false (Mã lỗi: %d)", errCode)
	}

	// Gọi RefreshData
	_, _ = oleutil.CallMethod(zkem, "RefreshData", dwMachineNumber)
	return nil
}
