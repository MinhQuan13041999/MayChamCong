package main

import (
	"fmt"
	"log"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

func main() {
	// 1. COM yêu cầu chạy trên cùng một OS Thread cố định để tránh các lỗi Threading
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Cấu hình thông tin kết nối thiết bị
	ipAddress := "192.168.11.151" // Thay đổi thành IP thật của máy chấm công của bạn
	port := int32(4370)          // Cổng mặc định ZKTeco

	fmt.Println("================================================================")
	fmt.Println("   MẪU KẾT NỐI MÁY CHẤM CÔNG ZKTECO DÙNG COM SDK (ZKemKeeper)   ")
	fmt.Println("================================================================")

	// 2. Khởi tạo OLE (COM library)
	err := ole.CoInitialize(0)
	if err != nil {
		log.Fatalf("Không thể khởi tạo OLE: %v", err)
	}
	defer ole.CoUninitialize()

	// 3. Khởi tạo đối tượng COM zkemkeeper
	// Class ID đăng ký trong Windows registry của ZKTeco COM SDK là "zkemkeeper.ZKEM" hoặc "zkemkeeper.ZKEM.1"
	unknown, err := oleutil.CreateObject("zkemkeeper.ZKEM.1")
	if err != nil {
		log.Fatalf("Không thể tạo đối tượng zkemkeeper. Lỗi: %v\n\n"+
			"LƯU Ý QUAN TRỌNG ĐỂ SỬ DỤNG COM SDK TRÊN WINDOWS:\n"+
			"1. Bạn phải đăng ký file zkemkeeper.dll vào hệ thống bằng lệnh CMD (quyền Administrator):\n"+
			"   regsvr32 C:\\path\\to\\zkemkeeper.dll\n"+
			"2. Đảm bảo kiến trúc ứng dụng Go (32-bit/64-bit) trùng khớp với dll đã đăng ký.\n"+
			"   - Nếu dùng Go x64: Đăng ký zkemkeeper.dll từ thư mục 64bit\n"+
			"   - Nếu dùng Go x86: Đăng ký zkemkeeper.dll từ thư mục 32bit\n", err)
	}
	defer unknown.Release()

	// 4. Lấy interface IDispatch để gọi các method
	zkem, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		log.Fatalf("Không thể query interface IDispatch: %v", err)
	}
	defer zkem.Release()

	// 5. Kết nối đến thiết bị qua TCP/IP
	fmt.Printf("[1] Đang kết nối tới thiết bị tại %s:%d...\n", ipAddress, port)

	// Method Connect_Net(IPAddr string, Port int) bool
	res, err := oleutil.CallMethod(zkem, "Connect_Net", ipAddress, port)
	if err != nil {
		log.Fatalf("Lỗi gọi hàm Connect_Net: %v", err)
	}

	connected := false
	if res != nil {
		val := res.Value()
		if b, ok := val.(bool); ok {
			connected = b
		} else if i, ok := val.(int32); ok {
			connected = i != 0
		} else if i, ok := val.(int64); ok {
			connected = i != 0
		}
	}

	if !connected {
		// Lấy mã lỗi từ thiết bị
		// Chú ý: GetLastError trong SDK ZK nhận tham số ByRef là mã lỗi
		var errCode int32
		vErrCode := ole.NewVariant(ole.VT_I4, 0)
		_, err = oleutil.CallMethod(zkem, "GetLastError", &vErrCode)
		if err == nil && vErrCode.Value() != nil {
			if code, ok := vErrCode.Value().(int32); ok {
				errCode = code
			}
		}
		vErrCode.Clear()
		log.Fatalf("Kết nối thất bại! Hãy kiểm tra IP, dây mạng và đảm bảo máy chấm công đang bật. Mã lỗi GetLastError: %d", errCode)
	}

	fmt.Println("=> Kết nối thành công!")

	// Đảm bảo ngắt kết nối khi kết thúc hàm main
	defer func() {
		_, _ = oleutil.CallMethod(zkem, "Disconnect")
		fmt.Println("=> Đã ngắt kết nối an toàn.")
	}()

	// 6. Lấy một số thông tin cơ bản của thiết bị
	var dwMachineNumber int32 = 1 // Mã máy (mặc định luôn là 1)

	// A. Lấy Serial Number
	vSN := ole.NewVariant(ole.VT_BSTR, 0)
	_, err = oleutil.CallMethod(zkem, "GetSerialNumber", dwMachineNumber, &vSN)
	if err == nil && vSN.Value() != nil {
		fmt.Printf("[2] Mã Serial Number của thiết bị: %s\n", vSN.Value().(string))
	} else {
		fmt.Printf("[2] Lỗi lấy Serial Number hoặc thiết bị không trả về: %v\n", err)
	}
	vSN.Clear()

	// B. Lấy phiên bản Card Reader (hoặc các phiên bản phần cứng khác nếu cần)
	vCardFun := ole.NewVariant(ole.VT_I4, 0)
	_, err = oleutil.CallMethod(zkem, "GetCardFun", dwMachineNumber, &vCardFun)
	if err == nil && vCardFun.Value() != nil {
		fmt.Printf("[3] Trạng thái hỗ trợ Thẻ: %v\n", vCardFun.Value())
	}
	vCardFun.Clear()

	// 7. Ví dụ đọc log chấm công (Attendance Logs)
	fmt.Println("[4] Đang nạp toàn bộ log chấm công từ thiết bị vào bộ nhớ đệm (cache) của SDK...")
	readLogRes, err := oleutil.CallMethod(zkem, "ReadGeneralLogData", dwMachineNumber)
	if err != nil {
		fmt.Printf("Lỗi gọi ReadGeneralLogData: %v\n", err)
		return
	}

	readLogSuccess := false
	if readLogRes != nil && readLogRes.Value() != nil {
		if val, ok := readLogRes.Value().(bool); ok {
			readLogSuccess = val
		}
	}

	if readLogSuccess {
		fmt.Println("=> Nạp dữ liệu thành công! Đang duyệt qua các log chấm công...")

		// Khai báo các variant làm đối số ByRef để nhận dữ liệu từ zkemkeeper
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

		count := 0
		// Duyệt qua từng log
		for {
			resLoop, err := oleutil.CallMethod(zkem, "SSR_GetGeneralLogData",
				dwMachineNumber,
				&vEnrollNumber,
				&vVerifyMode,
				&vInOutMode,
				&vYear,
				&vMonth,
				&vDay,
				&vHour,
				&vMinute,
				&vSecond,
				&vWorkCode,
			)
			if err != nil {
				fmt.Printf("Lỗi duyệt log: %v\n", err)
				break
			}

			// Nếu hàm trả về false thì tức là đã hết dữ liệu trong cache
			if resLoop == nil || resLoop.Value() == nil {
				break
			}
			valLoop, ok := resLoop.Value().(bool)
			if !ok || !valLoop {
				break
			}

			count++
			enrollNumber := ""
			if vEnrollNumber.Value() != nil {
				enrollNumber = vEnrollNumber.Value().(string)
			}
			
			var verifyMode, inOutMode, year, month, day, hour, minute, second int32
			if vVerifyMode.Value() != nil { verifyMode = vVerifyMode.Value().(int32) }
			if vInOutMode.Value() != nil { inOutMode = vInOutMode.Value().(int32) }
			if vYear.Value() != nil { year = vYear.Value().(int32) }
			if vMonth.Value() != nil { month = vMonth.Value().(int32) }
			if vDay.Value() != nil { day = vDay.Value().(int32) }
			if vHour.Value() != nil { hour = vHour.Value().(int32) }
			if vMinute.Value() != nil { minute = vMinute.Value().(int32) }
			if vSecond.Value() != nil { second = vSecond.Value().(int32) }

			checkTime := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", year, month, day, hour, minute, second)
			fmt.Printf("   Log #%d: Mã NV=%-10s | Thời gian=%s | VerifyMode=%d | InOutMode=%d\n",
				count, enrollNumber, checkTime, verifyMode, inOutMode)

			// Khống chế hiển thị 10 log làm ví dụ
			if count >= 10 {
				fmt.Println("   ... (chỉ hiển thị 10 log đầu tiên để làm mẫu)")
				break
			}
		}

		// Giải phóng tài nguyên các variant
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

		fmt.Printf("=> Tổng số log đọc được: %d\n", count)
	} else {
		fmt.Println("=> Không có log chấm công nào trên thiết bị.")
	}

	// 8. Hướng dẫn thêm một số hàm phổ biến khác qua COM SDK:
	/*
		// A. ĐỌC THÔNG TIN NHÂN VIÊN (SSR_GetAllUserInfo)
		// B1: Đọc thông tin nhân viên vào bộ nhớ đệm
		oleutil.CallMethod(zkem, "ReadAllUserID", dwMachineNumber)

		// B2: Duyệt thông tin từng nhân viên
		var (
			vEnrollNumber = ole.NewVariant(ole.VT_BSTR, 0)
			vName         = ole.NewVariant(ole.VT_BSTR, 0)
			vPassword     = ole.NewVariant(ole.VT_BSTR, 0)
			vPrivilege    = ole.NewVariant(ole.VT_I4, 0)
			vEnabled      = ole.NewVariant(ole.VT_BOOL, 0)
		)
		for {
			res, _ := oleutil.CallMethod(zkem, "SSR_GetAllUserInfo", dwMachineNumber, &vEnrollNumber, &vName, &vPassword, &vPrivilege, &vEnabled)
			if !res.Value().(bool) {
				break
			}
			fmt.Printf("Mã NV: %s, Tên: %s, Quyền: %d, Kích hoạt: %v\n",
				vEnrollNumber.Value().(string), vName.Value().(string), vPrivilege.Value().(int32), vEnabled.Value().(bool))
		}

		// B. THÊM / CẬP NHẬT NHÂN VIÊN (SSR_SetUserInfo)
		// SSR_SetUserInfo(dwMachineNumber int, dwEnrollNumber string, Name string, Password string, Privilege int, Enabled bool) bool
		ok, err := oleutil.CallMethod(zkem, "SSR_SetUserInfo", dwMachineNumber, "9001", "Nguyen Van A", "", 0, true)
	*/
}
