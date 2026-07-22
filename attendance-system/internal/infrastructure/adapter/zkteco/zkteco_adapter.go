// Package zkteco implements port.DeviceAdapter for ZKTeco devices via TCP/IP protocol.
//
// Kết nối chính: COM SDK ZKemKeeper (go-ole) qua Connect_Net — hỗ trợ mọi firmware ZKTeco.
// Fallback: github.com/canhlinh/gozk dùng cho GetAttendanceLogs nếu COM SDK không đọc được log.
package zkteco

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/canhlinh/gozk"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
)

// Adapter implement port.DeviceAdapter cho thiết bị ZKTeco qua TCP/IP.
// Sử dụng COM SDK (ZKemKeeper) làm giao thức chính; gozk làm fallback cho GetAttendanceLogs.
type Adapter struct {
	cfg              port.DeviceConfig
	zkClient         *gozk.ZK       // dùng cho GetAttendanceLogs fallback
	comConn          *comConnection // COM SDK session
	enrollmentActive bool           // terminal is currently running StartEnrollEx
	sessionGate      *deviceSessionGate
}

// ZKemKeeper is session-oriented: a second Connect_Net to the same terminal
// can take over the device state and cancel an active remote enrollment. Keep
// all adapter sessions for one IP:port serialized, including scheduler syncs.
type deviceSessionGate struct {
	token chan struct{}
}

var deviceSessionGates sync.Map // map[string]*deviceSessionGate

func sessionGateFor(ip string, port int) *deviceSessionGate {
	key := fmt.Sprintf("%s:%d", strings.ToLower(strings.TrimSpace(ip)), port)
	gate := &deviceSessionGate{token: make(chan struct{}, 1)}
	gate.token <- struct{}{}
	actual, _ := deviceSessionGates.LoadOrStore(key, gate)
	return actual.(*deviceSessionGate)
}

func (g *deviceSessionGate) acquire(ctx context.Context) error {
	select {
	case <-g.token:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *deviceSessionGate) release() {
	select {
	case g.token <- struct{}{}:
	default:
	}
}

// comConnection giữ trạng thái một phiên kết nối COM SDK đang mở.
type comConnection struct {
	unknown *ole.IUnknown
	zkem    *ole.IDispatch
	ip      string
	port    int
}

// initializeCOM treats S_FALSE as a successful COM initialization. Windows
// returns that code when the current thread was already initialized with the
// same apartment model; go-ole exposes it as an error with text "Incorrect
// function", which previously prevented otherwise valid SDK connections.
func initializeCOM() error {
	err := ole.CoInitialize(0)
	if err == nil {
		return nil
	}
	var oleErr *ole.OleError
	if errors.As(err, &oleErr) && oleErr.Code() == 1 { // S_FALSE
		return nil
	}
	return err
}

// New tạo instance mới cho ZKTeco adapter.
func New() *Adapter {
	return &Adapter{}
}

// Connect mở phiên kết nối đến máy chấm công qua COM SDK (ZKemKeeper).
// COM SDK hỗ trợ mọi dòng máy ZKTeco; gozk chỉ hỗ trợ một số model cũ.
func (a *Adapter) Connect(ctx context.Context, cfg port.DeviceConfig) error {
	a.cfg = cfg
	if cfg.IPAddress == "" {
		return fmt.Errorf("zkteco: ip_address is required")
	}

	devPort := cfg.Port
	if devPort == 0 {
		devPort = 4370
	}
	gate := sessionGateFor(cfg.IPAddress, devPort)
	if err := gate.acquire(ctx); err != nil {
		return fmt.Errorf("zkteco: waiting for device session: %w", err)
	}

	conn, err := openCOMConnection(cfg.IPAddress, devPort)
	if err != nil {
		gate.release()
		return fmt.Errorf("zkteco connect %s:%d: %w", cfg.IPAddress, devPort, err)
	}
	a.comConn = conn
	a.sessionGate = gate
	return nil
}

func (a *Adapter) Disconnect(ctx context.Context) error {
	if a.comConn != nil {
		// If this session owns an interactive enrollment, leave the terminal's
		// capture screen before closing the COM connection. This is what makes
		// cancelling an SDK batch take effect on the physical device.
		_, _ = oleutil.CallMethod(a.comConn.zkem, "CancelOperation")
		_, _ = oleutil.CallMethod(a.comConn.zkem, "StartIdentify")
		a.comConn.close()
		a.comConn = nil
	}
	if a.zkClient != nil {
		_ = a.zkClient.Disconnect()
		a.zkClient = nil
	}
	if a.sessionGate != nil {
		a.sessionGate.release()
		a.sessionGate = nil
	}
	return nil
}

func (a *Adapter) CheckStatus(ctx context.Context) (port.DeviceStatus, error) {
	if a.comConn == nil {
		return port.DeviceStatus{}, fmt.Errorf("zkteco: not connected")
	}
	zkem := a.comConn.zkem
	var dwMachineNumber int32 = 1

	// Lấy phiên bản firmware
	vFW := ole.NewVariant(ole.VT_BSTR, 0)
	defer vFW.Clear()
	_, err := oleutil.CallMethod(zkem, "GetFirmwareVersion", dwMachineNumber, &vFW)
	fw := ""
	if err == nil && vFW.Value() != nil {
		fw, _ = vFW.Value().(string)
	}

	// Lấy số người dùng
	vUserCount := ole.NewVariant(ole.VT_I4, 0)
	vFpCount := ole.NewVariant(ole.VT_I4, 0)
	vPwd := ole.NewVariant(ole.VT_I4, 0)
	vRecord := ole.NewVariant(ole.VT_I4, 0)
	vCamera := ole.NewVariant(ole.VT_I4, 0)
	defer vUserCount.Clear()
	defer vFpCount.Clear()
	defer vPwd.Clear()
	defer vRecord.Clear()
	defer vCamera.Clear()

	userCount := 0
	logCount := 0
	_, errProp := oleutil.CallMethod(zkem, "GetDeviceStatus",
		dwMachineNumber, int32(1), &vUserCount)
	if errProp == nil && vUserCount.Value() != nil {
		if v, ok := vUserCount.Value().(int32); ok {
			userCount = int(v)
		}
	}
	_, errLog := oleutil.CallMethod(zkem, "GetDeviceStatus",
		dwMachineNumber, int32(8), &vRecord)
	if errLog == nil && vRecord.Value() != nil {
		if v, ok := vRecord.Value().(int32); ok {
			logCount = int(v)
		}
	}

	return port.DeviceStatus{
		Online:       true,
		FirmwareInfo: fw,
		UserCount:    userCount,
		LogCount:     logCount,
	}, nil
}

// openCOMConnection khởi tạo COM và kết nối đến thiết bị.
// Caller phải gọi conn.close() sau khi dùng xong.
func openCOMConnection(ip string, port int) (*comConnection, error) {
	runtime.LockOSThread()
	// KHÔNG unlock ngay ở đây — OS thread phải giữ cho đến khi close()

	if err := initializeCOM(); err != nil {
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("ole CoInitialize: %w", err)
	}

	unknown, err := oleutil.CreateObject("zkemkeeper.ZKEM.1")
	if err != nil {
		ole.CoUninitialize()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("không thể tạo zkemkeeper.ZKEM.1 (hãy chạy regsvr32 zkemkeeper.dll với quyền Admin): %w", err)
	}

	zkem, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		unknown.Release()
		ole.CoUninitialize()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("query IDispatch: %w", err)
	}

	res, err := oleutil.CallMethod(zkem, "Connect_Net", ip, int32(port))
	if err != nil {
		zkem.Release()
		unknown.Release()
		ole.CoUninitialize()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("Connect_Net failed: %w", err)
	}

	connected := false
	if res != nil {
		switch v := res.Value().(type) {
		case bool:
			connected = v
		case int32:
			connected = v != 0
		}
	}

	if !connected {
		vErr := ole.NewVariant(ole.VT_I4, 0)
		_, _ = oleutil.CallMethod(zkem, "GetLastError", &vErr)
		var code int32
		if vErr.Value() != nil {
			code, _ = vErr.Value().(int32)
		}
		vErr.Clear()
		zkem.Release()
		unknown.Release()
		ole.CoUninitialize()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("kết nối tới %s:%d thất bại (GetLastError: %d) — kiểm tra IP, dây mạng và máy chấm công đang bật", ip, port, code)
	}

	return &comConnection{unknown: unknown, zkem: zkem, ip: ip, port: port}, nil
}

// close ngắt kết nối COM và giải phóng tài nguyên.
func (c *comConnection) close() {
	if c.zkem != nil {
		_, _ = oleutil.CallMethod(c.zkem, "Disconnect")
		c.zkem.Release()
		c.zkem = nil
	}
	if c.unknown != nil {
		c.unknown.Release()
		c.unknown = nil
	}
	ole.CoUninitialize()
	runtime.UnlockOSThread()
}

func (a *Adapter) SyncTime(ctx context.Context) error {
	if a.comConn == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	var dwMachineNumber int32 = 1
	now := time.Now()
	_, err := oleutil.CallMethod(a.comConn.zkem, "SetDeviceTime2",
		dwMachineNumber, int32(now.Year()), int32(now.Month()), int32(now.Day()),
		int32(now.Hour()), int32(now.Minute()), int32(now.Second()),
	)
	if err != nil {
		return fmt.Errorf("SetDeviceTime2 failed: %w", err)
	}
	return nil
}

// GetEmployees: đọc danh sách nhân viên từ máy chấm công sử dụng COM SDK.
func (a *Adapter) GetEmployees(ctx context.Context) ([]entity.Employee, error) {
	if a.comConn == nil {
		return nil, fmt.Errorf("zkteco: not connected")
	}
	emps, err := getEmployeesOnCOM(a.comConn.zkem)
	if err != nil {
		return nil, fmt.Errorf("zkteco: GetEmployees failed: %w", err)
	}
	return emps, nil
}

func (a *Adapter) PushEmployee(ctx context.Context, emp entity.Employee) error {
	if a.comConn == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	// Reuse the session opened by Connect. Opening a second Connect_Net session
	// while the first one is alive makes a number of terminals reject writes.
	if err := pushEmployeeOnCOM(a.comConn.zkem, emp); err != nil {
		return fmt.Errorf("zkteco: PushEmployee failed. Chi tiết lỗi COM: %w", err)
	}
	return nil
}

func (a *Adapter) PushFingerprint(ctx context.Context, employeeCode string, fp entity.EmployeeFingerprint) error {
	// The adapter opens the ZKTeco session through COM (comConn).  zkClient is
	// only the legacy attendance-log fallback and is normally nil, so checking
	// it here made every direct fingerprint push fail before SSR_SetUserTmpStr
	// could be called.
	if a.comConn == nil {
		return fmt.Errorf("zkteco: not connected")
	}

	// The COM dispatch object is apartment-thread bound. Use the connected
	// instance instead of creating a competing connection for every template.
	if err := pushFingerprintOnCOM(a.comConn.zkem, employeeCode, fp); err != nil {
		return fmt.Errorf("zkteco: PushFingerprint failed. Chi tiết lỗi COM: %w", err)
	}

	return nil
}

func (a *Adapter) GetFingerprint(ctx context.Context, employeeCode string, fingerIndex int) (*entity.EmployeeFingerprint, error) {
	if a.comConn == nil {
		return nil, fmt.Errorf("zkteco: not connected")
	}

	// Keep reads on the same COM connection. Opening another Connect_Net session
	// while an enrollment session is active causes many terminals to reject it.
	var fp *entity.EmployeeFingerprint
	var err error
	if a.enrollmentActive {
		// Cache reload operations can close the interactive enrollment screen.
		// The service leaves the terminal untouched for the complete scan window
		// before calling this method. Read the live value first, then leave the
		// enrollment state and refresh once only when that direct read did not
		// expose the newly persisted template.
		fp, err = getFingerprintDuringEnrollmentOnCOM(a.comConn.zkem, employeeCode, fingerIndex)
		a.enrollmentActive = false
		if err != nil || fp == nil || fp.TemplateData == "" {
			fp, err = getFingerprintOnCOM(a.comConn.zkem, employeeCode, fingerIndex)
		}
	} else {
		fp, err = getFingerprintOnCOM(a.comConn.zkem, employeeCode, fingerIndex)
	}
	if err != nil {
		return nil, fmt.Errorf("zkteco: GetFingerprint failed. Chi tiết lỗi COM: %w", err)
	}
	return fp, nil
}

// GetEmployeeFingerprints reads all non-empty templates for one user from a
// single COM cache load. Pulling code uses this instead of opening a fresh
// ReadAllTemplate operation for every finger index, which is both slower and
// disruptive on older ZKTeco firmware.
func (a *Adapter) GetEmployeeFingerprints(ctx context.Context, employeeCode string) ([]entity.EmployeeFingerprint, error) {
	all, err := a.GetAllEmployeeFingerprints(ctx, []string{employeeCode})
	if err != nil {
		return nil, err
	}
	return all[employeeCode], nil
}

// GetAllEmployeeFingerprints loads the SDK template cache once and returns all
// non-empty finger slots (indexes 0..9) for every requested user. Loading the
// cache once is important: reloading it for every user/finger is slow and can
// interfere with another SDK operation on older ZKTeco firmware.
func (a *Adapter) GetAllEmployeeFingerprints(ctx context.Context, employeeCodes []string) (map[string][]entity.EmployeeFingerprint, error) {
	const machineNumber int32 = 1
	if a.comConn == nil {
		return nil, fmt.Errorf("zkteco: not connected")
	}
	requested := make(map[string]struct{}, len(employeeCodes))
	for _, code := range employeeCodes {
		if trimmed := strings.TrimSpace(code); trimmed != "" {
			requested[trimmed] = struct{}{}
		}
	}

	_, _ = oleutil.CallMethod(a.comConn.zkem, "EnableDevice", machineNumber, false)
	defer func() { _, _ = oleutil.CallMethod(a.comConn.zkem, "EnableDevice", machineNumber, true) }()
	readUsers, readUsersErr := oleutil.CallMethod(a.comConn.zkem, "ReadAllUserID", machineNumber)
	if readUsersErr != nil || !comResultTrue(readUsers) {
		if readUsersErr != nil {
			return nil, fmt.Errorf("ReadAllUserID failed: %w", readUsersErr)
		}
		return nil, fmt.Errorf("ReadAllUserID returned false (GetLastError: %d)", comLastError(a.comConn.zkem))
	}
	readTemplates, readTemplatesErr := oleutil.CallMethod(a.comConn.zkem, "ReadAllTemplate", machineNumber)
	if readTemplatesErr != nil || !comResultTrue(readTemplates) {
		if readTemplatesErr != nil {
			return nil, fmt.Errorf("ReadAllTemplate failed: %w", readTemplatesErr)
		}
		return nil, fmt.Errorf("ReadAllTemplate returned false (GetLastError: %d)", comLastError(a.comConn.zkem))
	}

	result := make(map[string][]entity.EmployeeFingerprint, len(requested))
	for employeeCode := range requested {
		fingerprints := make([]entity.EmployeeFingerprint, 0, 10)
		for fingerIndex := 0; fingerIndex <= 9; fingerIndex++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			fp, err := getFingerprintOnCOMMode(a.comConn.zkem, employeeCode, fingerIndex, false)
			if err == nil && fp != nil && fp.TemplateData != "" {
				fingerprints = append(fingerprints, *fp)
			}
		}
		if len(fingerprints) > 0 {
			result[employeeCode] = fingerprints
		}
	}
	return result, nil
}

// pushEmployeeViaCOM thực hiện đẩy nhân viên xuống máy chấm công qua ZKTeco COM SDK (ZKemKeeper) qua go-ole.
func pushEmployeeOnCOM(zkem *ole.IDispatch, emp entity.Employee) error {
	const machineNumber int32 = 1
	if zkem == nil {
		return fmt.Errorf("COM session is not available")
	}

	result, err := oleutil.CallMethod(zkem, "SSR_SetUserInfo",
		machineNumber, emp.EmployeeCode, emp.FullName, "", int32(0), true)
	if err != nil {
		return fmt.Errorf("SSR_SetUserInfo failed: %w", err)
	}
	if !comResultTrue(result) {
		return fmt.Errorf("SSR_SetUserInfo returned false (GetLastError: %d)", comLastError(zkem))
	}
	if _, err := oleutil.CallMethod(zkem, "RefreshData", machineNumber); err != nil {
		return fmt.Errorf("RefreshData after SSR_SetUserInfo failed: %w", err)
	}
	return nil
}

// pushFingerprintOnCOM uses the method exposed by the bundled ZKemKeeper SDK:
// SetUserTmpExStr. SSR_SetUserTmpExStr is not a ZKemKeeper SDK method.
func pushFingerprintOnCOM(zkem *ole.IDispatch, employeeCode string, fp entity.EmployeeFingerprint) error {
	const machineNumber int32 = 1
	if zkem == nil {
		return fmt.Errorf("COM session is not available")
	}
	if employeeCode == "" {
		return fmt.Errorf("device user ID is required")
	}
	if fp.FingerIndex < 0 || fp.FingerIndex > 9 {
		return fmt.Errorf("invalid finger index %d", fp.FingerIndex)
	}
	if fp.TemplateData == "" {
		return fmt.Errorf("fingerprint template is empty")
	}

	_, _ = oleutil.CallMethod(zkem, "EnableDevice", machineNumber, false)
	defer func() { _, _ = oleutil.CallMethod(zkem, "EnableDevice", machineNumber, true) }()

	result, err := oleutil.CallMethod(zkem, "SetUserTmpExStr",
		machineNumber, employeeCode, int32(fp.FingerIndex), int32(1), fp.TemplateData)
	if err != nil {
		return fmt.Errorf("SetUserTmpExStr failed: %w", err)
	}
	if !comResultTrue(result) {
		return fmt.Errorf("SetUserTmpExStr returned false (GetLastError: %d)", comLastError(zkem))
	}
	if _, err := oleutil.CallMethod(zkem, "RefreshData", machineNumber); err != nil {
		return fmt.Errorf("RefreshData after SetUserTmpExStr failed: %w", err)
	}
	return nil
}

func comResultTrue(result *ole.VARIANT) bool {
	if result == nil {
		return false
	}
	switch value := result.Value().(type) {
	case bool:
		return value
	case int32:
		return value != 0
	case int:
		return value != 0
	default:
		return result.Val != 0
	}
}

func comLastError(zkem *ole.IDispatch) int32 {
	if zkem == nil {
		return 0
	}
	value := ole.NewVariant(ole.VT_I4, 0)
	defer value.Clear()
	if _, err := oleutil.CallMethod(zkem, "GetLastError", &value); err != nil {
		return 0
	}
	code, _ := value.Value().(int32)
	return code
}

func pushEmployeeViaCOM(ip string, port int, emp entity.Employee) error {
	// COM yêu cầu chạy trên cùng một OS Thread cố định
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := initializeCOM(); err != nil {
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
		"",       // Password rỗng
		int32(0), // Quyền user thường (0)
		true,     // Kích hoạt tài khoản (enabled)
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

	if err := initializeCOM(); err != nil {
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

	// SetUserTmpExStr ghi nhận template vân tay chuỗi Base64/Hex xuống máy
	resPush, err := oleutil.CallMethod(zkem, "SetUserTmpExStr",
		dwMachineNumber,
		employeeCode,
		int32(fp.FingerIndex),
		int32(1), // Flag = 1
		fp.TemplateData,
	)
	if err != nil {
		return fmt.Errorf("gọi SetUserTmpExStr thất bại: %w", err)
	}

	success := false
	if resPush.VT == ole.VT_BOOL {
		success = resPush.Value().(bool)
	} else {
		success = resPush.Val == 1 || resPush.Value().(bool)
	}

	if !success {
		return fmt.Errorf("SetUserTmpExStr trả về false (Mã lỗi: %d)", comLastError(zkem))
	}

	_, _ = oleutil.CallMethod(zkem, "RefreshData", dwMachineNumber)
	return nil
}

// GetAttendanceLogs đọc log bằng COM SDK và chuyển sang giao thức TCP SDK khi
// firmware không thực thi đúng SSR_GetGeneralLogData. Cả hai nhánh đều trả về
// cùng một mô hình AttendanceLog để tầng usecase và cơ chế chống trùng không
// bị thay đổi.
func (a *Adapter) GetAttendanceLogs(ctx context.Context, from, to time.Time) ([]entity.AttendanceLog, error) {
	if a.comConn == nil {
		return nil, fmt.Errorf("zkteco: not connected")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	zkem := a.comConn.zkem
	logs, comErr := readAttendanceLogsCOM(ctx, zkem, from, to)
	if comErr == nil {
		return logs, nil
	}

	// Chỉ rơi vào fallback TCP SDK (GoZK) khi COM SDK trả về lỗi thực sự
	if a.comConn != nil {
		a.comConn.close()
		a.comConn = nil
	}
	fallbackLogs, fallbackErr := readAttendanceLogsGoZK(ctx, a.cfg, from, to)
	if fallbackErr == nil {
		return fallbackLogs, nil
	}
	return nil, fmt.Errorf("COM SDK: %w; TCP SDK fallback: %v", comErr, fallbackErr)
}

func readAttendanceLogsCOM(ctx context.Context, zkem *ole.IDispatch, from, to time.Time) ([]entity.AttendanceLog, error) {
	if zkem == nil {
		return nil, fmt.Errorf("COM session is not available")
	}
	const machineNumber int32 = 1
	resRead, err := oleutil.CallMethod(zkem, "ReadGeneralLogData", machineNumber)
	if err != nil {
		return nil, fmt.Errorf("ReadGeneralLogData failed: %w", err)
	}
	if !comResultTrue(resRead) {
		errCode := comLastError(zkem)
		// Trên Firmware ZKTeco (như Ver 6.60), khi bộ nhớ log trống hoặc không có log mới,
		// ReadGeneralLogData trả về false với GetLastError = 0 hoặc 1.
		// Đây là trạng thái bình thường (0 log mới), không phải lỗi kết nối COM.
		if errCode == 0 || errCode == 1 || errCode == -100 {
			return nil, nil
		}
		return nil, fmt.Errorf("ReadGeneralLogData returned false (GetLastError: %d)", errCode)
	}

	var (
		vEnrollNumber = ole.NewVariant(ole.VT_BSTR, 0)
		vVerifyMode   = ole.NewVariant(ole.VT_I4, 0)
		vInOutMode    = ole.NewVariant(ole.VT_I4, 0)
		vYear         = ole.NewVariant(ole.VT_I4, 0)
		vMonth        = ole.NewVariant(ole.VT_I4, 0)
		vDay          = ole.NewVariant(ole.VT_I4, 0)
		vHour         = ole.NewVariant(ole.VT_I4, 0)
		vMinute       = ole.NewVariant(ole.VT_I4, 0)
		vSecond       = ole.NewVariant(ole.VT_I4, 0)
		vWorkCode     = ole.NewVariant(ole.VT_I4, 0)
	)
	defer func() {
		vEnrollNumber.Clear()
		vVerifyMode.Clear()
		vInOutMode.Clear()
		vYear.Clear()
		vMonth.Clear()
		vDay.Clear()
		vHour.Clear()
		vMinute.Clear()
		vSecond.Clear()
		vWorkCode.Clear()
	}()

	var logs []entity.AttendanceLog
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		resLoop, err := oleutil.CallMethod(zkem, "SSR_GetGeneralLogData",
			machineNumber, &vEnrollNumber, &vVerifyMode, &vInOutMode,
			&vYear, &vMonth, &vDay, &vHour, &vMinute, &vSecond, &vWorkCode,
		)
		if err != nil {
			return nil, fmt.Errorf("SSR_GetGeneralLogData failed: %w", err)
		}
		if !comResultTrue(resLoop) {
			break // false means end of the SDK buffer, not an error.
		}

		enrollNumber := variantString(&vEnrollNumber)
		year, yearOK := variantInt32(&vYear)
		month, monthOK := variantInt32(&vMonth)
		day, dayOK := variantInt32(&vDay)
		hour, hourOK := variantInt32(&vHour)
		minute, minuteOK := variantInt32(&vMinute)
		second, secondOK := variantInt32(&vSecond)
		inOutMode, _ := variantInt32(&vInOutMode)
		verifyMode, _ := variantInt32(&vVerifyMode)
		if enrollNumber == "" || !yearOK || !monthOK || !dayOK || !hourOK || !minuteOK || !secondOK {
			return nil, fmt.Errorf("SSR_GetGeneralLogData returned an invalid row (pin=%q)", enrollNumber)
		}

		checkTime := time.Date(int(year), time.Month(month), int(day), int(hour), int(minute), int(second), 0, time.Local)
		if checkTime.Before(from) || checkTime.After(to) {
			continue
		}
		logs = append(logs, buildSDKAttendanceLog(enrollNumber, checkTime, inOutMode, verifyMode, "com_sdk"))
	}
	return logs, nil
}

func readAttendanceLogsGoZK(ctx context.Context, cfg port.DeviceConfig, from, to time.Time) (logs []entity.AttendanceLog, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("TCP SDK panic while reading logs: %v", recovered)
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	portNumber := cfg.Port
	if portNumber == 0 {
		portNumber = 4370
	}
	zk := gozk.NewZK(cfg.IPAddress,
		gozk.WithPort(portNumber),
		gozk.WithTCP(true),
		gozk.WithTimezone("Asia/Ho_Chi_Minh"),
		gozk.WithDeviceID(cfg.IPAddress),
	)
	if err := zk.Connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer zk.Disconnect()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	events, err := zk.GetAllScannedEvents()
	if err != nil {
		return nil, fmt.Errorf("GetAllScannedEvents failed: %w", err)
	}
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if event == nil || event.Error != nil || event.UserID < 0 || event.Timestamp.IsZero() {
			continue
		}
		checkTime := event.Timestamp.In(time.Local)
		if checkTime.Before(from) || checkTime.After(to) {
			continue
		}
		logs = append(logs, buildSDKAttendanceLog(strconv.FormatInt(event.UserID, 10), checkTime, 0, 1, "tcp_sdk"))
	}
	return logs, nil
}

func buildSDKAttendanceLog(enrollNumber string, checkTime time.Time, inOutMode, verifyMode int32, source string) entity.AttendanceLog {
	checkType := entity.CheckTypeIn
	if inOutMode == 1 {
		checkType = entity.CheckTypeOut
	}
	verifyModeStr := entity.VerifyModeFingerprint
	if verifyMode == 3 || verifyMode == 4 {
		verifyModeStr = entity.VerifyModeCard
	} else if verifyMode == 15 || verifyMode == 20 {
		verifyModeStr = entity.VerifyModeFace
	}
	rawBytes, _ := json.Marshal(map[string]interface{}{
		"enroll_number": enrollNumber,
		"verify_mode":   verifyMode,
		"in_out_mode":   inOutMode,
		"timestamp":     checkTime.Format("2006-01-02 15:04:05"),
		"source":        source,
	})
	return entity.AttendanceLog{EmployeeCode: enrollNumber, CheckTime: checkTime, CheckType: checkType, VerifyMode: verifyModeStr, RawPayload: rawBytes}
}

func comDeviceRecordCount(zkem *ole.IDispatch) (int, bool) {
	if zkem == nil {
		return 0, false
	}
	vRecord := ole.NewVariant(ole.VT_I4, 0)
	defer vRecord.Clear()
	result, err := oleutil.CallMethod(zkem, "GetDeviceStatus", int32(1), int32(8), &vRecord)
	if err != nil || !comResultTrue(result) {
		return 0, false
	}
	value, ok := variantInt32(&vRecord)
	return int(value), ok
}

func variantString(value *ole.VARIANT) string {
	if value == nil || value.Value() == nil {
		return ""
	}
	return strings.TrimRight(fmt.Sprint(value.Value()), "\x00 ")
}

func variantInt32(value *ole.VARIANT) (int32, bool) {
	if value == nil || value.Value() == nil {
		return 0, false
	}
	switch v := value.Value().(type) {
	case int32:
		return v, true
	case int:
		return int32(v), true
	case int16:
		return int32(v), true
	case int64:
		return int32(v), true
	case uint32:
		return int32(v), true
	case float64:
		return int32(v), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 32)
		return int32(parsed), err == nil
	default:
		return 0, false
	}
}

func (a *Adapter) ClearAttendanceLogs(ctx context.Context) error {
	if a.comConn == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	return clearLogsViaCOM(a.cfg.IPAddress, a.cfg.Port)
}

// ClearEmployees is deliberately not implemented through the generic adapter.
// ClearData with the ZKTeco user-data flag also removes enrolled templates, so
// exposing it here would make an ordinary API call destructive.
func (a *Adapter) ClearEmployees(ctx context.Context) error {
	return fmt.Errorf("zkteco: ClearEmployees is not supported; use a dedicated, confirmed device reset operation")
}

func (a *Adapter) Reboot(ctx context.Context) error {
	if a.comConn == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	// Reuse the COM connection opened by Connect. Opening a second session just
	// for reboot can make the terminal reject the command as already connected.
	return restartDeviceOnCOM(a.comConn.zkem)
}

func (a *Adapter) Reset(ctx context.Context) error {
	if a.comConn == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	return resetViaCOM(a.cfg.IPAddress, a.cfg.Port)
}

func (a *Adapter) EnrollFingerprint(ctx context.Context, employeeCode string, fingerIndex int) error {
	if a.comConn == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	if err := enrollFingerprintOnCOM(a.comConn.zkem, employeeCode, fingerIndex); err != nil {
		return err
	}
	a.enrollmentActive = true
	return nil
}

// DeleteEmployee removes one user from the terminal through the ZKemKeeper
// COM SDK.  It intentionally targets a single user; bulk deletion remains a
// separately confirmed operation because it also removes templates.
func (a *Adapter) DeleteEmployee(ctx context.Context, employeeCode string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.comConn == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	if strings.TrimSpace(employeeCode) == "" {
		return fmt.Errorf("device user ID is required")
	}
	const machineNumber int32 = 1
	result, err := oleutil.CallMethod(a.comConn.zkem, "SSR_DeleteUserInfo", machineNumber, employeeCode)
	if err != nil {
		return fmt.Errorf("SSR_DeleteUserInfo failed: %w", err)
	}
	if !comResultTrue(result) {
		return fmt.Errorf("SSR_DeleteUserInfo returned false (GetLastError: %d)", comLastError(a.comConn.zkem))
	}
	_, _ = oleutil.CallMethod(a.comConn.zkem, "RefreshData", machineNumber)
	return nil
}

// DeleteFingerprint removes one template slot from the terminal through the
// same COM session used for enrollment.
func (a *Adapter) DeleteFingerprint(ctx context.Context, employeeCode string, fingerIndex int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.comConn == nil {
		return fmt.Errorf("zkteco: not connected")
	}
	if strings.TrimSpace(employeeCode) == "" {
		return fmt.Errorf("device user ID is required")
	}
	if fingerIndex < 0 || fingerIndex > 9 {
		return fmt.Errorf("invalid finger index %d", fingerIndex)
	}
	const machineNumber int32 = 1
	result, err := oleutil.CallMethod(a.comConn.zkem, "SSR_DelUserTmpExt", machineNumber, employeeCode, int32(fingerIndex))
	if err != nil {
		return fmt.Errorf("SSR_DelUserTmpExt failed: %w", err)
	}
	if !comResultTrue(result) {
		return fmt.Errorf("SSR_DelUserTmpExt returned false (GetLastError: %d)", comLastError(a.comConn.zkem))
	}
	_, _ = oleutil.CallMethod(a.comConn.zkem, "RefreshData", machineNumber)
	return nil
}

// getFingerprintViaCOM thực hiện đọc template vân tay từ máy chấm công qua ZKTeco COM SDK (ZKemKeeper) qua go-ole.
func getFingerprintOnCOM(zkem *ole.IDispatch, employeeCode string, fingerIndex int) (*entity.EmployeeFingerprint, error) {
	return getFingerprintOnCOMMode(zkem, employeeCode, fingerIndex, true)
}

func getFingerprintDuringEnrollmentOnCOM(zkem *ole.IDispatch, employeeCode string, fingerIndex int) (*entity.EmployeeFingerprint, error) {
	return getFingerprintOnCOMMode(zkem, employeeCode, fingerIndex, false)
}

func getFingerprintOnCOMMode(zkem *ole.IDispatch, employeeCode string, fingerIndex int, refreshCache bool) (*entity.EmployeeFingerprint, error) {
	const machineNumber int32 = 1
	if zkem == nil {
		return nil, fmt.Errorf("COM session is not available")
	}
	if employeeCode == "" {
		return nil, fmt.Errorf("device user ID is required")
	}
	if fingerIndex < 0 || fingerIndex > 9 {
		return nil, fmt.Errorf("invalid finger index %d", fingerIndex)
	}
	if refreshCache {
		// Normal pull/download reads refresh the device cache. Never do this
		// while StartEnrollEx owns the terminal's capture screen.
		// ZKTeco's SDK samples disable the terminal while loading the user and
		// template caches; without this, some firmware returns users correctly
		// but leaves every template lookup empty.
		_, _ = oleutil.CallMethod(zkem, "EnableDevice", machineNumber, false)
		defer func() { _, _ = oleutil.CallMethod(zkem, "EnableDevice", machineNumber, true) }()
		_, _ = oleutil.CallMethod(zkem, "ReadAllUserID", machineNumber)
		_, _ = oleutil.CallMethod(zkem, "ReadAllTemplate", machineNumber)
	}

	var flag int32
	var template string
	var templateSize int32
	result, err := oleutil.CallMethod(zkem, "GetUserTmpExStr", machineNumber, employeeCode, int32(fingerIndex), &flag, &template, &templateSize)
	if err == nil && comResultTrue(result) {
		if template != "" {
			return &entity.EmployeeFingerprint{FingerIndex: fingerIndex, TemplateData: template, TemplateSize: int(templateSize), AlgoVersion: "10.0"}, nil
		}
	}

	// Older 9.0 devices do not implement GetUserTmpExStr. Their SDK sample
	// uses this legacy method instead.
	var legacyTemplate string
	var legacySize int32
	legacyResult, legacyErr := oleutil.CallMethod(zkem, "SSR_GetUserTmpStr", machineNumber, employeeCode, int32(fingerIndex), &legacyTemplate, &legacySize)
	if legacyErr == nil && comResultTrue(legacyResult) {
		if legacyTemplate != "" {
			return &entity.EmployeeFingerprint{FingerIndex: fingerIndex, TemplateData: legacyTemplate, TemplateSize: int(legacySize), AlgoVersion: "9.0"}, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("GetUserTmpExStr failed: %w", err)
	}
	return nil, fmt.Errorf("fingerprint %d is not available for user %s", fingerIndex, employeeCode)
}

func getFingerprintViaCOM(ip string, port int, employeeCode string, fingerIndex int) (*entity.EmployeeFingerprint, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := initializeCOM(); err != nil {
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
func restartDeviceOnCOM(zkem *ole.IDispatch) error {
	const machineNumber int32 = 1
	if zkem == nil {
		return fmt.Errorf("COM session is not available")
	}

	// RestartDevice is the method exposed by ZKemKeeper and used by the
	// official IFace, TFT and Black&White SDK samples. Some third-party type
	// libraries expose RebootDevice instead, so keep that name as a fallback.
	result, err := oleutil.CallMethod(zkem, "RestartDevice", machineNumber)
	method := "RestartDevice"
	if err != nil {
		fallbackResult, fallbackErr := oleutil.CallMethod(zkem, "RebootDevice", machineNumber)
		if fallbackErr != nil {
			return fmt.Errorf("RestartDevice failed: %v; RebootDevice fallback failed: %w", err, fallbackErr)
		}
		result = fallbackResult
		method = "RebootDevice"
	}
	if !comResultTrue(result) {
		return fmt.Errorf("%s returned false (GetLastError: %d)", method, comLastError(zkem))
	}
	return nil
}

func rebootViaCOM(ip string, port int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := initializeCOM(); err != nil {
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
	resReboot, err := oleutil.CallMethod(zkem, "RestartDevice", dwMachineNumber)
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

	if err := initializeCOM(); err != nil {
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

	if err := initializeCOM(); err != nil {
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
// getEmployeesOnCOM loads users from the existing, apartment-bound ZKEM
// connection. A second Connect_Net session can leave TFT user enumeration
// empty even though the first session is connected successfully.
func getEmployeesOnCOM(zkem *ole.IDispatch) ([]entity.Employee, error) {
	const machineNumber int32 = 1
	if zkem == nil {
		return nil, fmt.Errorf("COM session is not available")
	}

	// Match ZKTeco's UserInfo sample: disable the terminal while loading both
	// caches, then enumerate the string user IDs through SSR_GetAllUserInfo.
	_, _ = oleutil.CallMethod(zkem, "EnableDevice", machineNumber, false)
	defer func() { _, _ = oleutil.CallMethod(zkem, "EnableDevice", machineNumber, true) }()

	resRead, err := oleutil.CallMethod(zkem, "ReadAllUserID", machineNumber)
	if err != nil {
		return nil, fmt.Errorf("ReadAllUserID failed: %w", err)
	}
	if !comResultTrue(resRead) {
		return nil, fmt.Errorf("ReadAllUserID returned false (GetLastError: %d)", comLastError(zkem))
	}
	_, _ = oleutil.CallMethod(zkem, "ReadAllTemplate", machineNumber)

	employees := make([]entity.Employee, 0)
	for {
		var enrollNumber string
		var name string
		var password string
		var privilege int32
		var enabled bool
		result, err := oleutil.CallMethod(zkem, "SSR_GetAllUserInfo", machineNumber, &enrollNumber, &name, &password, &privilege, &enabled)
		if err != nil {
			return getEmployeesLegacyOnCOM(zkem)
		}
		if !comResultTrue(result) {
			break
		}

		code := strings.TrimSpace(enrollNumber)
		if code == "" {
			continue
		}
		employees = append(employees, entity.Employee{
			EmployeeCode: code,
			FullName:     strings.TrimSpace(name),
			Status:       "active",
		})
	}
	if len(employees) == 0 {
		// IFace/older firmware exposes only the numeric user API. Try it when
		// the string SSR enumeration is unavailable or empty.
		return getEmployeesLegacyOnCOM(zkem)
	}
	return employees, nil
}

func getEmployeesLegacyOnCOM(zkem *ole.IDispatch) ([]entity.Employee, error) {
	const machineNumber int32 = 1
	employees := make([]entity.Employee, 0)
	for {
		var userID int32
		var name string
		var password string
		var privilege int32
		var enabled bool
		result, err := oleutil.CallMethod(zkem, "GetAllUserInfo", machineNumber, &userID, &name, &password, &privilege, &enabled)
		if err != nil {
			return nil, fmt.Errorf("GetAllUserInfo fallback failed: %w", err)
		}
		if !comResultTrue(result) {
			break
		}
		employees = append(employees, entity.Employee{
			EmployeeCode: strconv.FormatInt(int64(userID), 10),
			FullName:     strings.TrimSpace(name),
			Status:       "active",
		})
	}
	return employees, nil
}

func getEmployeesViaCOM(ip string, port int) ([]entity.Employee, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := initializeCOM(); err != nil {
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
func enrollFingerprintOnCOM(zkem *ole.IDispatch, employeeCode string, fingerIndex int) error {
	const machineNumber int32 = 1
	if zkem == nil {
		return fmt.Errorf("COM session is not available")
	}
	if employeeCode == "" {
		return fmt.Errorf("device user ID is required")
	}
	if fingerIndex < 0 || fingerIndex > 9 {
		return fmt.Errorf("invalid finger index %d", fingerIndex)
	}

	// Clear a previous unfinished operation, then enter the terminal's native
	// fingerprint-capture screen on the same SDK connection.
	_, _ = oleutil.CallMethod(zkem, "CancelOperation")
	_, _ = oleutil.CallMethod(zkem, "SSR_DelUserTmpExt", machineNumber, employeeCode, int32(fingerIndex))
	result, err := oleutil.CallMethod(zkem, "StartEnrollEx", employeeCode, int32(fingerIndex), int32(1))
	if err != nil {
		return fmt.Errorf("StartEnrollEx failed: %w", err)
	}
	if !comResultTrue(result) {
		return fmt.Errorf("StartEnrollEx returned false (GetLastError: %d)", comLastError(zkem))
	}
	// ZKemKeeper's online-enrollment sample enters identify mode immediately
	// after StartEnrollEx. Without this call, several TFT/IFace firmwares show
	// the enrollment page briefly and return to idle before a finger can be read.
	identifyResult, identifyErr := oleutil.CallMethod(zkem, "StartIdentify")
	if identifyErr != nil {
		return fmt.Errorf("StartIdentify after StartEnrollEx failed: %w", identifyErr)
	}
	if !comResultTrue(identifyResult) {
		return fmt.Errorf("StartIdentify after StartEnrollEx returned false (GetLastError: %d)", comLastError(zkem))
	}
	// Do not call RefreshData, ReadAllUserID or ReadAllTemplate while the
	// terminal is capturing; those cache operations can close the screen.
	return nil
}

func enrollFingerprintViaCOM(ip string, port int, employeeCode string, fingerIndex int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := initializeCOM(); err != nil {
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
