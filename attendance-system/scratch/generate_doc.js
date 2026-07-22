const {
    Document,
    Packer,
    Paragraph,
    TextRun,
    HeadingLevel,
    Table,
    TableRow,
    TableCell,
    WidthType,
    AlignmentType,
    PageBreak
} = require("docx");
const fs = require("fs");
const path = require("path");

// Colors
const COLOR_PRIMARY = "0f2027";    // Dark Navy
const COLOR_SECONDARY = "203a43";  // Slate Teal
const COLOR_ACCENT = "2c5364";     // Muted Blue
const COLOR_TEXT = "333333";       // Charcoal

function createTitle(text) {
    return new Paragraph({
        heading: HeadingLevel.TITLE,
        alignment: AlignmentType.CENTER,
        spacing: { before: 240, after: 120 },
        children: [
            new TextRun({
                text: text,
                bold: true,
                size: 52, // 26pt
                font: "Arial",
                color: COLOR_PRIMARY,
            }),
        ],
    });
}

function createSubtitle(text) {
    return new Paragraph({
        alignment: AlignmentType.CENTER,
        spacing: { before: 120, after: 360 },
        children: [
            new TextRun({
                text: text,
                size: 28, // 14pt
                font: "Arial",
                color: COLOR_SECONDARY,
                italic: true,
            }),
        ],
    });
}

function createHeading1(text) {
    return new Paragraph({
        heading: HeadingLevel.HEADING_1,
        spacing: { before: 400, after: 200 },
        keepWithNext: true,
        children: [
            new TextRun({
                text: text,
                bold: true,
                size: 32, // 16pt
                font: "Arial",
                color: COLOR_PRIMARY,
            }),
        ],
    });
}

function createHeading2(text) {
    return new Paragraph({
        heading: HeadingLevel.HEADING_2,
        spacing: { before: 240, after: 120 },
        keepWithNext: true,
        children: [
            new TextRun({
                text: text,
                bold: true,
                size: 26, // 13pt
                font: "Arial",
                color: COLOR_SECONDARY,
            }),
        ],
    });
}

function createHeading3(text) {
    return new Paragraph({
        heading: HeadingLevel.HEADING_3,
        spacing: { before: 180, after: 60 },
        keepWithNext: true,
        children: [
            new TextRun({
                text: text,
                bold: true,
                size: 22, // 11pt
                font: "Arial",
                color: COLOR_ACCENT,
                italic: true,
            }),
        ],
    });
}

function createBodyText(text, options = {}) {
    return new Paragraph({
        spacing: { before: 100, after: 100 },
        children: [
            new TextRun({
                text: text,
                bold: options.bold || false,
                italic: options.italic || false,
                size: 22, // 11pt
                font: "Arial",
                color: options.color || COLOR_TEXT,
            }),
        ],
    });
}

function createBulletPoint(text, level = 0) {
    return new Paragraph({
        bullet: {
            level: level,
        },
        spacing: { before: 50, after: 50 },
        children: [
            new TextRun({
                text: text,
                size: 22,
                font: "Arial",
                color: COLOR_TEXT,
            }),
        ],
    });
}

function createTableHeaderCell(text, widthPercent) {
    return new TableCell({
        children: [
            new Paragraph({
                alignment: AlignmentType.LEFT,
                children: [
                    new TextRun({
                        text: text,
                        bold: true,
                        color: "ffffff",
                        size: 20,
                        font: "Arial",
                    }),
                ],
            }),
        ],
        shading: {
            fill: COLOR_PRIMARY,
        },
        width: {
            size: widthPercent,
            type: WidthType.PERCENTAGE,
        },
        margins: {
            top: 120,
            bottom: 120,
            left: 150,
            right: 150,
        },
    });
}

function createTableCell(text, widthPercent, options = {}) {
    return new TableCell({
        children: [
            new Paragraph({
                alignment: AlignmentType.LEFT,
                children: [
                    new TextRun({
                        text: text,
                        bold: options.bold || false,
                        italic: options.italic || false,
                        size: 20,
                        font: "Arial",
                        color: COLOR_TEXT,
                    }),
                ],
            }),
        ],
        shading: options.bg ? { fill: options.bg } : undefined,
        width: {
            size: widthPercent,
            type: WidthType.PERCENTAGE,
        },
        margins: {
            top: 120,
            bottom: 120,
            left: 150,
            right: 150,
        },
    });
}

function createDBTable(title, columns) {
    const rows = [
        new TableRow({
            children: [
                createTableHeaderCell("Tên cột", 30),
                createTableHeaderCell("Kiểu dữ liệu & Ràng buộc", 35),
                createTableHeaderCell("Mô tả chi tiết vai trò", 35),
            ]
        })
    ];
    for (const col of columns) {
        rows.push(new TableRow({
            children: [
                createTableCell(col.name, 30, { bold: col.pk || false }),
                createTableCell(col.type, 35, { italic: col.pk || false }),
                createTableCell(col.desc, 35),
            ]
        }));
    }
    return new Table({
        width: { size: 100, type: WidthType.PERCENTAGE },
        rows: rows
    });
}

// Generate the doc
const doc = new Document({
    sections: [{
        properties: {},
        children: [
            // --- TRANG BÌA ---
            new Paragraph({ spacing: { before: 1800 } }),
            createTitle("BÁO CÁO KỸ THUẬT VÀ BÀN GIAO DỰ ÁN"),
            createSubtitle("Hệ Thống Chấm Công Doanh Nghiệp Lớn (Enterprise Attendance System)"),
            new Paragraph({ spacing: { before: 800 } }),
            new Paragraph({
                alignment: AlignmentType.CENTER,
                children: [
                    new TextRun({
                        text: "ĐỐI TƯỢNG BÀN GIAO: Khách hàng / Hội đồng Đánh giá / Nội bộ Doanh nghiệp\n",
                        bold: true,
                        size: 24,
                        font: "Arial",
                        color: COLOR_SECONDARY,
                    }),
                    new TextRun({
                        text: "BÁO CÁO ĐẶC TẢ CHI TIẾT CÁC TÍNH NĂNG VÀ CẤU TRÚC KỸ THUẬT HỆ THỐNG\n",
                        italic: true,
                        size: 20,
                        font: "Arial",
                        color: COLOR_ACCENT,
                    }),
                    new TextRun({
                        text: "NGƯỜI THỰC HIỆN: Nhóm Phát triển Dự án\n",
                        size: 22,
                        font: "Arial",
                        color: COLOR_TEXT,
                    }),
                    new TextRun({
                        text: "MÃ PHIÊN BẢN: v2.5.0-Enterprise\n",
                        bold: true,
                        size: 20,
                        font: "Arial",
                        color: COLOR_PRIMARY,
                    }),
                ],
            }),
            new PageBreak(),

            // --- CHƯƠNG 1 ---
            createHeading1("CHƯƠNG 1: TỔNG QUAN DỰ ÁN & ĐẶC TẢ CHI TIẾT TÍNH NĂNG CỐT LÕI"),
            
            createHeading2("1.1. Lý do chọn đề tài & Sự cần thiết của Hệ thống"),
            createBodyText("Trong kỷ nguyên chuyển đổi số, việc quản lý thời gian làm việc và chấm công của nhân sự đóng vai trò cốt lõi trong vận hành doanh nghiệp. Đối với các tổ chức lớn có quy mô từ hàng trăm đến hàng ngàn nhân sự, trải dài trên nhiều chi nhánh và cơ sở sản xuất, việc chấm công thủ công hoặc sử dụng các phần mềm rời rạc bộc lộ nhiều điểm nghẽn nghiêm trọng:"),
            createBulletPoint("Sự phụ thuộc vào một nhà cung cấp thiết bị phần cứng: Doanh nghiệp không thể tận dụng đồng thời các dòng máy cũ (như ZKTeco qua SDK kết nối mạng nội bộ) và các dòng máy nhận diện khuôn mặt mới (như Hikvision, Sunbeam) trên cùng một hệ thống quản lý."),
            createBulletPoint("Quản lý dữ liệu sinh trắc học phân tán: Vân tay hoặc khuôn mặt của nhân viên đăng ký tại máy chấm công này không thể tự động đồng bộ sang máy chấm công ở chi nhánh khác, dẫn đến việc nhân sự phải đăng ký lại nhiều lần khi di chuyển địa điểm làm việc."),
            createBulletPoint("Thiếu quy trình Tự phục vụ (ESS - Employee Self Service): Khi nhân viên quên quẹt thẻ, xin nghỉ phép, hoặc đi công tác, họ phải thực hiện quy trình giấy tờ thủ công phức tạp, gây quá tải cho bộ phận Hành chính Nhân sự (HR)."),
            createBulletPoint("Bài toán tính công phức tạp: Việc tính toán công ngày thủ công dựa trên log quẹt thẻ thô rất dễ sai sót, đặc biệt là các ca làm việc đặc thù như ca đêm kéo dài qua ngày hôm sau (Overnight Shift), ca gãy, hoặc đi muộn về sớm có thời gian ân hạn (Grace Minutes)."),
            createBodyText("Để giải quyết triệt để các vấn đề trên, dự án \"Hệ thống chấm công doanh nghiệp lớn (Enterprise Attendance System)\" được phát triển nhằm cung cấp một giải pháp quản lý tập trung, tự động hóa toàn bộ quy trình từ khâu thu thập log phần cứng đa hãng đến tính toán công nghiệp vụ và thông báo tức thời cho người lao động."),

            createHeading2("1.2. Mục tiêu của Dự án"),
            createBulletPoint("Xây dựng hệ thống backend hiệu năng cao bằng ngôn ngữ Go (Golang) có khả năng kết nối song song và xử lý hàng triệu bản ghi chấm công từ hàng trăm thiết bị đầu cuối."),
            createBulletPoint("Tích hợp thành công đa dạng giao thức kết nối phần cứng: zkemkeeper.dll COM SDK (ZKTeco Standalone), ADMS Push Protocol (ZKTeco tự đẩy dữ liệu qua HTTP), ISAPI REST (Hikvision) và REST API HTTP (Sunbeam)."),
            createBulletPoint("Thiết lập kho lưu trữ vân tay tập trung tại database PostgreSQL và xây dựng cơ chế tự động phân phối mẫu vân tay xuống các thiết bị đích, hỗ trợ quản lý tương thích phiên bản thuật toán vân tay."),
            createBulletPoint("Tự động hóa hoàn toàn quy trình phê duyệt đơn từ (nghỉ phép, làm thêm giờ, sửa giờ công) và đồng bộ trực tiếp kết quả vào dữ liệu tính công ngày."),
            createBulletPoint("Gia tăng trải nghiệm nhân viên thông qua các kênh thông báo tức thời (gửi email, tin nhắn Zalo OA khi quẹt thẻ) và cảnh báo nhắc nhở quên check-out tự động."),

            createHeading2("1.3. Phạm vi Dự án & Đối tượng sử dụng"),
            createHeading3("Phạm vi hệ thống:"),
            createBulletPoint("Hệ thống phần mềm chạy tập trung tại Cloud Server hoặc Server nội bộ của doanh nghiệp."),
            createBulletPoint("Giao diện quản trị trên Web UI dành cho Ban Giám đốc, Quản lý bộ phận và nhân viên HR."),
            createBulletPoint("Dịch vụ chạy ngầm (Background Workers / Schedulers) thực hiện kết nối thiết bị phần cứng vật lý qua mạng WAN/LAN."),
            createBulletPoint("Kênh tích hợp ngoại vi: SMTP Server của doanh nghiệp, Zalo OA API, và cổng thông tin SSE (Server-Sent Events) đẩy log chấm công real-time."),
            
            createHeading3("Đối tượng sử dụng hệ thống:"),
            createBulletPoint("Ban Giám đốc / Quản trị viên hệ thống (Admin): Giám sát toàn bộ trạng thái hoạt động của các thiết bị chấm công, quản lý cấu hình hệ thống, kiểm toán hành động (Audit Logs)."),
            createBulletPoint("Nhân viên Hành chính Nhân sự (HR): Quản lý hồ sơ nhân sự, phân ca làm việc, phê duyệt đơn từ nghỉ phép/tăng ca/sửa giờ công, xuất báo cáo chấm công tháng ra file Excel để phục vụ tính lương."),
            createBulletPoint("Người lao động (Employee): Xem ca làm việc cá nhân, gửi đơn từ tự phục vụ (ESS), nhận thông báo quẹt thẻ và cảnh báo quên check-out tức thời."),

            createHeading2("1.4. Bảng Đặc tả Chi tiết 10 Module Cốt lõi của Hệ thống"),
            createBodyText("Dưới đây là bảng đặc tả chi tiết toàn bộ các module chức năng đã được triển khai thực tế trong mã nguồn của dự án:"),

            new Table({
                width: { size: 100, type: WidthType.PERCENTAGE },
                rows: [
                    new TableRow({
                        children: [
                            createTableHeaderCell("Mã Module", 15),
                            createTableHeaderCell("Tên Phân hệ", 25),
                            createTableHeaderCell("Tính năng & Mô tả Nghiệp vụ Chi tiết", 60),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("MOD-01", 15, { bold: true }),
                            createTableCell("Xác thực & Phân quyền", 25, { bold: true }),
                            createTableCell("• Xác thực người dùng thông qua mã hóa mật khẩu một chiều bcrypt.\n• Cấp phát và quản lý phiên làm việc bằng JSON Web Token (JWT Bearer Token) thời hạn 24h.\n• Hỗ trợ xác thực kép LDAP/Active Directory tích hợp hệ thống tài khoản doanh nghiệp sẵn có.\n• Phân quyền chi tiết dựa trên vai trò (RBAC): Admin, HR, Viewer kết hợp bảng ánh xạ quyền chi tiết (permission).", 60),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("MOD-02", 15, { bold: true }),
                            createTableCell("Quản lý Thiết bị chấm công", 25, { bold: true }),
                            createTableCell("• Khai báo thông tin thiết bị: Tên máy, hãng (ZKTeco, Hikvision, Sunbeam), giao thức kết nối, IP, Port, Username, Password, Serial Number, vị trí đặt máy.\n• Kiểm tra trạng thái trực tuyến (Test Connection) của máy.\n• Gửi các lệnh điều khiển từ xa: Reboot máy, Reset máy, Xóa sạch log thô (Clear Logs) để giải phóng bộ nhớ đệm.", 60),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("MOD-03", 15, { bold: true }),
                            createTableCell("Quản lý Nhân sự & Ánh xạ", 25, { bold: true }),
                            createTableCell("• Quản lý hồ sơ nhân viên: Mã nhân viên, họ tên, phòng ban, email, điện thoại, giới tính, ngày sinh, chức danh, ảnh đại diện, Zalo User ID.\n• Import hồ sơ nhân viên hàng loạt từ file Excel (.xlsx).\n• Đồng bộ thông tin nhân sự xuống một thiết bị cụ thể hoặc toàn bộ thiết bị đang online.\n• Kéo thông tin nhân viên đăng ký trực tiếp trên máy chấm công về database server.\n• Xóa tài khoản nhân viên trên thiết bị chấm công từ xa.", 60),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("MOD-04", 15, { bold: true }),
                            createTableCell("Quản lý Vân tay tập trung", 25, { bold: true }),
                            createTableCell("• Sao lưu toàn bộ mẫu vân tay (từ ngón 0 đến ngón 9) của nhân viên từ máy chấm công nguồn về database dưới dạng Base64 (bảng employee_fingerprint).\n• Phân phối và đẩy mẫu vân tay đã sao lưu xuống các máy chấm công đích khác.\n• Kiểm tra và tự động chuyển đổi tương thích phiên bản thuật toán vân tay (9.0 hoặc 10.0) của thiết bị nhận.\n• Kích hoạt chế độ đăng ký vân tay từ xa (Remote Enroll) trực tiếp từ màn hình Web UI.\n• Chức năng đăng ký vân tay hàng loạt theo Wizard (Batch Enroll) tự động bỏ qua nếu nhân viên không thực hiện quét vân tay sau 10 giây.", 60),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("MOD-05", 15, { bold: true }),
                            createTableCell("Khai báo Ca & Phân ca", 25, { bold: true }),
                            createTableCell("• Khai báo ca làm việc linh hoạt: Giờ bắt đầu, giờ kết thúc, thời gian nghỉ giải lao (BreakMinutes), số phút đi muộn cho phép (LateGraceMinutes), số phút về sớm cho phép (EarlyGraceMinutes).\n• Gán ca làm việc (Assign Shift) cho nhân viên theo chu kỳ bắt đầu - kết thúc cụ thể.\n• Hỗ trợ cấu hình màu sắc ca làm việc (ColorCode) để quản trị lịch trực quan trên Web UI.", 60),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("MOD-06", 15, { bold: true }),
                            createTableCell("Đơn từ & Tự phục vụ (ESS)", 25, { bold: true }),
                            createTableCell("• Cung cấp giao diện tự phục vụ cho nhân viên để gửi yêu cầu nghỉ phép (Leave Request), tăng ca (Overtime Request), hoặc giải trình sửa giờ chấm công (Attendance Correction).\n• Quy trình duyệt đơn tự động: Khi đơn sửa giờ công được HR duyệt, hệ thống tự sinh bản ghi log ảo tương ứng để bù giờ.\n• Quản lý trạng thái đơn từ: pending | approved | rejected kèm lịch sử người duyệt.", 60),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("MOD-07", 15, { bold: true }),
                            createTableCell("Đồng bộ Log & Khử trùng", 25, { bold: true }),
                            createTableCell("• Tự động đồng bộ log chấm công định kỳ (mỗi 10 giây) từ các máy Standalone và Hikvision/Sunbeam qua background scheduler.\n• Tiếp nhận dữ liệu chấm công tự động đẩy lên qua cổng ADMS HTTP POST của dòng máy ADMS.\n• Giải thuật khử trùng lặp dữ liệu (Deduplication) tại database PostgreSQL dựa trên khóa UNIQUE (device_id, employee_code, check_time) nhằm loại bỏ log thừa khi chạy song song cả cơ chế kéo và đẩy.", 60),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("MOD-08", 15, { bold: true }),
                            createTableCell("Tính toán công tự động", 25, { bold: true }),
                            createTableCell("• Engine tính công tự động quét log thô đầu/cuối ngày chấm công của nhân sự.\n• So khớp dữ liệu quẹt thẻ thực tế với ca làm việc được phân để tính: số giờ làm việc thực tế (working_hours), số phút đi muộn (late_minutes), số phút về sớm (early_minutes), trạng thái công ngày (present | absent | late | early | leave).\n• Giải thuật tính ca đêm qua ngày (Overnight Shift): Tự động gom log quẹt tối ngày hôm trước và sáng ngày hôm sau vào cùng một dòng công.\n• Tự động bù trừ phép (gắn kết leave_request) và tính giờ tăng ca (gắn kết overtime_request).", 60),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("MOD-09", 15, { bold: true }),
                            createTableCell("Báo cáo & Xuất Excel", 25, { bold: true }),
                            createTableCell("• Kết xuất ma trận bảng công tổng hợp tháng của toàn bộ nhân viên.\n• Xuất file báo cáo Excel (.xlsx) chuyên nghiệp bằng thư viện Excelize tốc độ cao.\n• Cho phép HR gửi trực tiếp file Excel bảng công chi tiết cá nhân tới hòm thư của từng nhân viên.", 60),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("MOD-10", 15, { bold: true }),
                            createTableCell("SSE & Cảnh báo tức thời", 25, { bold: true }),
                            createTableCell("• Đẩy log chấm công real-time hiển thị trực tiếp lên màn hình Web UI giám sát của admin qua giao thức Server-Sent Events (SSE).\n• Tự động gửi email thông báo chấm công và tin nhắn Zalo OA cho nhân viên ngay khi họ quẹt vân tay/khuôn mặt thành công trên máy chấm công.\n• Worker chạy ngầm mỗi phút tự kiểm tra nhân viên hết ca làm việc quá 30 phút mà quên check-out để gửi tin nhắn nhắc nhở (Checkout Reminder).", 60),
                        ],
                    }),
                ],
            }),

            new PageBreak(),

            // --- CHƯƠNG 2 ---
            createHeading1("CHƯƠNG 2: KIẾN TRÚC DỰ ÁN & SƠ ĐỒ LUỒNG DỮ LIỆU CHUYÊN SÂU"),
            
            createHeading2("2.1. Phân tích chi tiết Kiến trúc Clean Architecture trong Go"),
            createBodyText("Hệ thống tuân thủ nghiêm ngặt nguyên lý thiết kế Clean Architecture của Robert C. Martin, phân chia mã nguồn thành 4 vòng tròn đồng tâm độc lập. Mục tiêu cốt lõi là làm cho logic nghiệp vụ hoàn toàn độc lập với database, UI framework và các thư viện kết nối phần cứng."),
            
            createHeading3("Cấu trúc phân mục và vai trò chi tiết:"),
            createBulletPoint("Thư mục cmd/server: Chứa entrypoint main.go của ứng dụng. Nhiệm vụ chính là khởi tạo các kết nối cơ sở dữ liệu (PostgreSQL Connection Pool), đọc tham số cấu hình từ config.yaml hoặc biến môi trường, đăng ký dependency injection và khởi động server HTTP Gin cùng scheduler chạy nền."),
            createBulletPoint("Thư mục internal/domain: Là trung tâm của kiến trúc (Core), định nghĩa các struct thực thể nghiệp vụ (như Device, Employee, AttendanceLog, Shift, DailyAttendance...) và định nghĩa các interface (Port) cho tầng lưu trữ (Repository) và tầng adapter phần cứng (DeviceAdapter). Tầng này độc lập hoàn toàn và không import bất kỳ thư viện ngoài nào (như gorm, sql, hay driver thiết bị)."),
            createBulletPoint("Thư mục internal/usecase: Chứa đựng toàn bộ các luật nghiệp vụ (Business Rules). Ví dụ: file attendance_processor_service.go chứa thuật toán phân tích log, tính giờ trễ/sớm và ghép cặp ca đêm; biometric_service.go điều phối việc đăng ký vân tay và đẩy lệnh hàng đợi xuống máy ADMS. Tầng này chỉ tương tác với các interface ở lớp Domain thông qua dependency injection."),
            createBulletPoint("Thư mục internal/infrastructure: Chứa code chi tiết về công nghệ (Adapter). Bao gồm thư mục postgres/ (kết nối vật lý DB bằng pgxpool, thực thi các câu lệnh SQL), zkteco/ (triển khai COM SDK zkemkeeper.dll qua go-ole), hikvision/ (gọi API HTTP Digest REST của hãng Hikvision), sunbeam/ (REST API) và scheduler/ (quản lý cron ticker 10 giây/lần)."),
            createBulletPoint("Thư mục internal/interface/http: Tiếp nhận các request HTTP từ Web UI client. Bao gồm router cấu hình định tuyến API, handlers (nhận request DTO, kiểm tra hợp lệ đầu vào, chuyển tiếp xuống Usecase, định dạng output JSON trả về) và phân hệ SSE (Server-Sent Events) đẩy log real-time."),

            createHeading2("2.2. Thiết kế trừu tượng hóa phần cứng (Hardware Abstraction Layer)"),
            createBodyText("Để giải quyết triệt để bài toán tích hợp đa chủng loại thiết bị và không phụ thuộc nhà cung cấp, hệ thống thiết kế lớp trừu tượng thông qua interface DeviceAdapter ở tầng Domain:"),
            createBodyText(`type DeviceAdapter interface {
    TestConnection(ctx context.Context, dev *entity.Device) error
    GetAttendanceLogs(ctx context.Context, dev *entity.Device, fromTime time.Time) ([]entity.AttendanceLog, error)
    SyncEmployee(ctx context.Context, dev *entity.Device, emp *entity.Employee) error
    DeleteEmployee(ctx context.Context, dev *entity.Device, employeeCode string) error
    EnrollFingerprint(ctx context.Context, dev *entity.Device, employeeCode string, fingerIndex int) error
    ClearEnrollData(ctx context.Context, dev *entity.Device) error
}`),
            createBodyText("Khi hệ thống khởi chạy hoặc nhận yêu cầu tương tác thiết bị, lớp Factory (DeviceAdapterFactory) sẽ đọc trường device_type của thiết bị lưu dưới DB để trả về instance tương ứng:"),
            createBulletPoint("zkteco (Standalone): Khởi tạo adapter kết nối COM SDK ActiveX (zkemkeeper.dll) chạy trên Windows qua cầu nối go-ole."),
            createBulletPoint("zkteco (ADMS): Trả về adapter tương tác thông qua hàng đợi lệnh (device_command_queue) gửi phản hồi HTTP cho máy chấm công ADMS."),
            createBulletPoint("hikvision: Trả về adapter gọi API ISAPI qua HTTP REST sử dụng Digest Authentication."),
            createBulletPoint("sunbeam: Trả về adapter REST API chuẩn HTTP JSON."),

            createHeading2("2.3. Sơ đồ Luồng Dữ liệu (Data Flow) Đồng bộ Dual-Sync"),
            createBodyText("Để đảm bảo log chấm công được kéo về ngay lập tức nhưng vẫn có tính dự phòng và an toàn khi kết nối mạng chập chờn, hệ thống kết hợp 2 luồng đồng bộ chạy song song:"),
            
            createHeading3("Luồng 1: Push Protocol - Máy chấm công tự đẩy dữ liệu (ZKTeco ADMS)"),
            createBulletPoint("Bước 1: Nhân viên quẹt vân tay hoặc nhận diện khuôn mặt thành công trên máy chấm công ADMS."),
            createBulletPoint("Bước 2: Thiết bị đóng gói bản ghi log và gửi request HTTP POST /iclock/cdata?SN=...&table=ATTLOG lên Go server."),
            createBulletPoint("Bước 3: Lớp Interface HTTP (ADMS Handler) tiếp nhận request, parse dữ liệu text thô của ADMS thành mảng thực thể AttendanceLog."),
            createBulletPoint("Bước 4: Go server lưu bản ghi vào database Postgres. Nhờ cấu hình UNIQUE, log trùng lặp sẽ bị loại bỏ."),
            createBulletPoint("Bước 5: Phát tín hiệu SSE đẩy log lên màn hình dashboard giám sát của admin."),
            createBulletPoint("Bước 6: Spawn một Goroutine chạy nền gọi NotificationService để gửi tin nhắn thông báo tức thời cho nhân sự qua Zalo OA và Email."),
            createBulletPoint("Bước 7: Thiết bị ADMS tiếp tục gọi HTTP GET /iclock/getrequest?SN=... để lấy các lệnh chờ trong hàng đợi (device_command_queue) như Enroll, Delete, Reboot... và thực thi."),

            createHeading3("Luồng 2: Pull Protocol - Quét chủ động (Standalone SDK & Hikvision/Sunbeam)"),
            createBulletPoint("Bước 1: Scheduler trong nhân hệ thống chạy nền định kỳ mỗi 10 giây."),
            createBulletPoint("Bước 2: Scheduler gọi SyncService thực hiện lặp qua danh sách thiết bị có adms_enabled = false."),
            createBulletPoint("Bước 3: Với mỗi thiết bị, đọc mốc thời gian đồng bộ gần nhất từ bảng sync_cursor để xác định điểm bắt đầu quét log mới."),
            createBulletPoint("Bước 4: Kết nối vật lý tới thiết bị (ZKTeco SDK hoặc HTTP Digest Hikvision), thực hiện tải log chấm công phát sinh sau mốc cursor."),
            createBulletPoint("Bước 5: Ghi toàn bộ log tải về vào database PostgreSQL."),
            createBulletPoint("Bước 6: Nếu thành công, cập nhật con trỏ sync_cursor lên mốc thời gian của log mới nhất vừa tải để tối ưu lượng băng thông truyền tải cho chu kỳ quét tiếp theo."),

            createHeading2("2.4. Thuật toán Khử trùng lặp Dữ liệu & Giải thuật Tính công Ca đêm chuyên sâu"),
            
            createHeading3("2.4.1. Cơ chế Khử trùng lặp Log (Log Deduplication)"),
            createBodyText("Do hệ thống cho phép chạy song song cả cơ chế kéo chủ động (Pull) và thiết bị tự đẩy (ADMS Push), nguy cơ trùng lặp dữ liệu log chấm công là rất lớn. Để loại bỏ triệt để, hệ thống áp dụng cơ chế khử trùng lặp 2 lớp:"),
            createBulletPoint("Lớp 1 (Ràng buộc cứng DB): Thiết lập index độc nhất UNIQUE(device_id, employee_code, check_time) trên bảng attendance_log. Khi chèn dữ liệu, nếu trùng lặp sẽ bị cơ chế ON CONFLICT DO NOTHING của PostgreSQL loại bỏ trực tiếp."),
            createBulletPoint("Lớp 2 (Kiểm tra phần mềm): Tầng Usecase trước khi chèn hàng loạt (Bulk Insert) sẽ đối chiếu với log hiện hữu trong khoảng thời gian scan để lọc bỏ bản ghi cũ trước khi gửi xuống DB."),

            createHeading3("2.4.2. Giải thuật Tính công Ca đêm qua ngày (Overnight Shift Algorithm)"),
            createBodyText("Bài toán: Nhân viên ca đêm bắt đầu check-in từ 21:00 tối hôm trước và check-out lúc 06:00 sáng ngày hôm sau. Nếu phân tích theo ngày dương lịch thông thường, hệ thống sẽ ghi nhận ngày hôm trước thiếu giờ check-out và ngày hôm sau thiếu giờ check-in, dẫn đến tính sai công hoàn toàn."),
            createBodyText("Giải thuật xử lý trong mã nguồn hệ thống:"),
            createBulletPoint("Bước 1: Đọc lịch gán ca (employee_shift) của nhân viên tại ngày bắt đầu quẹt thẻ (Ngày D). Xác định xem ca đó có phải là ca đêm hay không (giờ kết thúc nhỏ hơn giờ bắt đầu)."),
            createBulletPoint("Bước 2: Tìm lượt check-in đầu tiên (First In) của nhân viên trong ngày D nằm trong khung thời gian ân hạn bắt đầu ca (ví dụ: từ 19:00 đến 23:00)."),
            createBulletPoint("Bước 3: Thiết lập cửa sổ tìm kiếm check-out (Check-out Window) kéo dài từ giờ Check-In đến tối đa MaxWorkingMinutes của ca gán (ví dụ: ca đêm 8 tiếng, MaxWorkingMinutes là 840 phút = 14 tiếng). Cửa sổ tìm kiếm Check-Out sẽ kéo dài tới 11:00 trưa ngày hôm sau (Ngày D+1)."),
            createBulletPoint("Bước 4: Quét toàn bộ log chấm công thô của nhân viên trong cửa sổ tìm kiếm trên. Lượt quẹt cuối cùng tìm thấy sẽ được gán làm giờ Check-Out (Last Out) cho ngày công D."),
            createBulletPoint("Bước 5: Tổng hợp số giờ làm việc thực tế = (Last Out - First In) - BreakMinutes. Cập nhật trạng thái công ngày D là 'present' và ghi nhận vào bảng daily_attendance của ngày D. Toàn bộ log quẹt thẻ của ca đêm này sẽ không bị tách sang ngày D+1."),

            new PageBreak(),

            // --- CHƯƠNG 3 ---
            createHeading1("CHƯƠNG 3: THIẾT KẾ CƠ SỞ DỮ LIỆU & CHI TIẾT BẢNG LƯU VẾT"),
            createBodyText("Cơ sở dữ liệu PostgreSQL được thiết kế chặt chẽ, tối ưu hiệu năng thông qua các index và ràng buộc khóa ngoại cascade để tự động đồng bộ trạng thái khi xóa dữ liệu."),
            
            createHeading2("3.1. Danh sách chi tiết toàn bộ các Bảng cơ sở dữ liệu (19 Bảng)"),

            createHeading3("Bảng 1: department (Quản lý Phòng Ban)"),
            createDBTable("department", [
                { name: "id", type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()", desc: "Khóa chính duy nhất của phòng ban", pk: true },
                { name: "name", type: "VARCHAR(255) NOT NULL", desc: "Tên phòng ban" },
                { name: "code", type: "VARCHAR(50) UNIQUE", desc: "Mã phòng ban" },
                { name: "parent_id", type: "UUID REFERENCES department(id) ON DELETE SET NULL", desc: "Liên kết phòng ban cấp cha (tính năng phân cấp)" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Thời điểm tạo bản ghi" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 2: employee (Quản lý Nhân sự)"),
            createDBTable("employee", [
                { name: "id", type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()", desc: "Khóa chính nhân viên", pk: true },
                { name: "employee_code", type: "VARCHAR(50) UNIQUE NOT NULL", desc: "Mã số nhân viên (dùng đồng bộ với máy chấm công)" },
                { name: "full_name", type: "VARCHAR(255) NOT NULL", desc: "Họ và tên đầy đủ" },
                { name: "department_id", type: "UUID REFERENCES department(id) ON DELETE SET NULL", desc: "Mã phòng ban trực thuộc" },
                { name: "card_no", type: "VARCHAR(50)", desc: "Mã thẻ từ chấm công" },
                { name: "fingerprint_enrolled", type: "BOOLEAN DEFAULT false", desc: "Trạng thái đã đăng ký vân tay hay chưa" },
                { name: "face_enrolled", type: "BOOLEAN DEFAULT false", desc: "Trạng thái đã đăng ký nhận diện khuôn mặt" },
                { name: "status", type: "VARCHAR(20) DEFAULT 'active'", desc: "Trạng thái nhân sự: active | inactive" },
                { name: "email", type: "VARCHAR(100) UNIQUE", desc: "Email cá nhân phục vụ nhận thông báo" },
                { name: "phone", type: "VARCHAR(20)", desc: "Số điện thoại liên hệ" },
                { name: "zalo_user_id", type: "VARCHAR(100)", desc: "ID tài khoản Zalo cá nhân phục vụ gửi tin nhắn OA" },
                { name: "gender", type: "VARCHAR(10) CHECK CHECK(gender IN ('male', 'female', 'other'))", desc: "Giới tính nhân sự" },
                { name: "dob", type: "DATE", desc: "Ngày sinh" },
                { name: "join_date", type: "DATE DEFAULT CURRENT_DATE", desc: "Ngày bắt đầu làm việc" },
                { name: "job_title", type: "VARCHAR(100)", desc: "Chức danh / Vị trí công việc" },
                { name: "avatar_url", type: "VARCHAR(255)", desc: "Đường dẫn ảnh đại diện nhân viên" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Thời điểm tạo nhân viên" },
                { name: "updated_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Thời điểm cập nhật gần nhất" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 3: device (Quản lý thiết bị chấm công)"),
            createDBTable("device", [
                { name: "id", type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()", desc: "Khóa chính thiết bị", pk: true },
                { name: "name", type: "VARCHAR(255) NOT NULL", desc: "Tên máy hiển thị trên Web" },
                { name: "device_type", type: "VARCHAR(20) NOT NULL", desc: "Hãng sản xuất: zkteco | sunbeam | hikvision" },
                { name: "ip_address", type: "VARCHAR(50) NOT NULL", desc: "Địa chỉ IP tĩnh của máy chấm công" },
                { name: "port", type: "INTEGER NOT NULL", desc: "Port kết nối" },
                { name: "serial_number", type: "VARCHAR(100)", desc: "Số Serial của thiết bị (SDK)" },
                { name: "serial_number_adms", type: "VARCHAR(100) UNIQUE", desc: "Số Serial dùng cho giao thức tự đẩy ADMS" },
                { name: "adms_enabled", type: "BOOLEAN NOT NULL DEFAULT false", desc: "Cho phép thiết bị tự động đẩy log" },
                { name: "status", type: "VARCHAR(20) DEFAULT 'offline'", desc: "Trạng thái online / offline" },
                { name: "last_checked_at", type: "TIMESTAMPTZ", desc: "Thời điểm ping kiểm tra gần nhất" },
                { name: "last_heartbeat_at", type: "TIMESTAMPTZ", desc: "Thời điểm nhận heartbeat ADMS gần nhất" },
                { name: "location", type: "VARCHAR(255)", desc: "Vị trí đặt máy chấm công" },
                { name: "firmware_version", type: "VARCHAR(50)", desc: "Phiên bản firmware máy" },
                { name: "last_online_at", type: "TIMESTAMPTZ", desc: "Thời gian online cuối cùng" },
                { name: "mac_address", type: "VARCHAR(50) UNIQUE", desc: "Địa chỉ vật lý MAC" },
                { name: "username", type: "VARCHAR(100) DEFAULT ''", desc: "Tài khoản đăng nhập API của thiết bị" },
                { name: "password", type: "VARCHAR(100) DEFAULT ''", desc: "Mật khẩu đăng nhập API của thiết bị" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Thời điểm khai báo máy" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 4: attendance_log (Nhật ký chấm công thô)"),
            createDBTable("attendance_log", [
                { name: "id", type: "BIGSERIAL PRIMARY KEY", desc: "Mã log tự tăng", pk: true },
                { name: "device_id", type: "UUID REFERENCES device(id) ON DELETE SET NULL", desc: "Mã máy ghi nhận lần chấm công này" },
                { name: "employee_code", type: "VARCHAR(50) NOT NULL", desc: "Mã nhân viên lưu trên máy chấm công" },
                { name: "check_time", type: "TIMESTAMPTZ NOT NULL", desc: "Thời gian quẹt thẻ/vân tay" },
                { name: "check_type", type: "VARCHAR(20)", desc: "Loại chấm công: in | out | unknown" },
                { name: "verify_mode", type: "VARCHAR(20)", desc: "Cách thức: fingerprint | face | card" },
                { name: "raw_payload", type: "JSONB", desc: "Lưu dữ liệu thô nhận về phục vụ đối chiếu" },
                { name: "synced_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Thời điểm đồng bộ về server" }
            ]),
            createBodyText("Lưu ý: Bảng này có ràng buộc UNIQUE(device_id, employee_code, check_time) để tránh trùng log."),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 5: sync_history (Lịch sử đồng bộ thiết bị)"),
            createDBTable("sync_history", [
                { name: "id", type: "BIGSERIAL PRIMARY KEY", desc: "Mã lịch sử", pk: true },
                { name: "device_id", type: "UUID REFERENCES device(id) ON DELETE CASCADE", desc: "Mã thiết bị thực hiện đồng bộ" },
                { name: "sync_type", type: "VARCHAR(30)", desc: "Loại đồng bộ: employee | attendance | time_sync" },
                { name: "trigger_type", type: "VARCHAR(20)", desc: "Hình thức kích hoạt: manual | scheduled" },
                { name: "status", type: "VARCHAR(20)", desc: "Kết quả: success | failed | partial" },
                { name: "record_count", type: "INTEGER DEFAULT 0", desc: "Số lượng bản ghi xử lý" },
                { name: "error_message", type: "TEXT", desc: "Nội dung lỗi nếu đồng bộ thất bại" },
                { name: "started_at", type: "TIMESTAMPTZ", desc: "Thời điểm bắt đầu" },
                { name: "finished_at", type: "TIMESTAMPTZ", desc: "Thời điểm kết thúc" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 6: shift (Cấu hình ca làm việc)"),
            createDBTable("shift", [
                { name: "id", type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()", desc: "Khóa chính ca làm việc", pk: true },
                { name: "name", type: "VARCHAR(100) NOT NULL", desc: "Tên ca (ví dụ: Ca hành chính, Ca đêm)" },
                { name: "start_time", type: "TIME NOT NULL", desc: "Giờ bắt đầu làm việc" },
                { name: "end_time", type: "TIME NOT NULL", desc: "Giờ kết thúc ca làm việc" },
                { name: "break_minutes", type: "INTEGER DEFAULT 0", desc: "Số phút giải lao được trừ" },
                { name: "late_grace_minutes", type: "INTEGER DEFAULT 0", desc: "Số phút cho phép đi muộn không tính trễ" },
                { name: "early_grace_minutes", type: "INTEGER DEFAULT 0", desc: "Số phút cho phép về sớm không tính phạt" },
                { name: "max_working_minutes", type: "INTEGER DEFAULT 0", desc: "Số phút làm việc tối đa trong ca" },
                { name: "timezone", type: "VARCHAR(64) DEFAULT 'Asia/Ho_Chi_Minh'", desc: "Múi giờ áp dụng cho ca" },
                { name: "color_code", type: "VARCHAR(7) DEFAULT '#4F46E5'", desc: "Màu hiển thị trên lịch Web" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Ngày tạo ca làm việc" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 7: employee_shift (Gán ca làm việc cho nhân viên)"),
            createDBTable("employee_shift", [
                { name: "id", type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()", desc: "Mã gán ca", pk: true },
                { name: "employee_id", type: "UUID REFERENCES employee(id) ON DELETE CASCADE", desc: "Mã nhân viên nhận ca" },
                { name: "shift_id", type: "UUID REFERENCES shift(id) ON DELETE CASCADE", desc: "Mã ca được gán" },
                { name: "start_date", type: "DATE NOT NULL", desc: "Ngày bắt đầu áp dụng" },
                { name: "end_date", type: "DATE", desc: "Ngày kết thúc áp dụng (NULL nếu lâu dài)" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Thời điểm tạo" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 8: leave_request (Đăng ký nghỉ phép)"),
            createDBTable("leave_request", [
                { name: "id", type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()", desc: "Mã đơn nghỉ phép", pk: true },
                { name: "employee_id", type: "UUID REFERENCES employee(id) ON DELETE CASCADE", desc: "Mã nhân viên xin nghỉ" },
                { name: "leave_type", type: "VARCHAR(50) NOT NULL", desc: "Loại phép: annual | sick | unpaid | business_trip" },
                { name: "start_date", type: "DATE NOT NULL", desc: "Ngày xin nghỉ bắt đầu" },
                { name: "end_date", type: "DATE NOT NULL", desc: "Ngày xin nghỉ kết thúc" },
                { name: "reason", type: "TEXT", desc: "Lý do xin nghỉ" },
                { name: "status", type: "VARCHAR(20) DEFAULT 'pending'", desc: "Trạng thái: pending | approved | rejected" },
                { name: "approved_by", type: "UUID REFERENCES \"user\"(id) ON DELETE SET NULL", desc: "Mã admin/HR phê duyệt đơn" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Ngày gửi đơn" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 9: daily_attendance (Bảng công tổng hợp ngày)"),
            createDBTable("daily_attendance", [
                { name: "id", type: "BIGSERIAL PRIMARY KEY", desc: "Mã công ngày", pk: true },
                { name: "employee_id", type: "UUID REFERENCES employee(id) ON DELETE CASCADE", desc: "Mã nhân viên" },
                { name: "date", type: "DATE NOT NULL", desc: "Ngày làm việc" },
                { name: "shift_id", type: "UUID REFERENCES shift(id) ON DELETE SET NULL", desc: "Ca làm việc thực tế áp dụng" },
                { name: "first_in", type: "TIMESTAMPTZ", desc: "Giờ Check-In thực tế đầu tiên" },
                { name: "last_out", type: "TIMESTAMPTZ", desc: "Giờ Check-Out thực tế cuối cùng" },
                { name: "late_minutes", type: "INTEGER DEFAULT 0", desc: "Số phút đi muộn" },
                { name: "early_minutes", type: "INTEGER DEFAULT 0", desc: "Số phút về sớm" },
                { name: "working_hours", type: "NUMERIC(4,2) DEFAULT 0.00", desc: "Tổng số giờ làm việc quy đổi" },
                { name: "attendance_status", type: "VARCHAR(30) DEFAULT 'absent'", desc: "Trạng thái công: present | absent | late | early | leave" },
                { name: "overtime_minutes", type: "INTEGER DEFAULT 0", desc: "Số phút tăng ca (OT) được duyệt" },
                { name: "leave_id", type: "UUID REFERENCES leave_request(id) ON DELETE SET NULL", desc: "Mã đơn nghỉ phép liên đới (nếu có)" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Thời điểm ghi nhận" },
                { name: "updated_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Thời điểm cập nhật" }
            ]),
            createBodyText("Lưu ý: Bảng này có ràng buộc UNIQUE(employee_id, date) để đảm bảo không bị trùng công ngày."),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 10: overtime_request (Đăng ký tăng ca)"),
            createDBTable("overtime_request", [
                { name: "id", type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()", desc: "Mã đơn đăng ký OT", pk: true },
                { name: "employee_id", type: "UUID REFERENCES employee(id) ON DELETE CASCADE", desc: "Mã nhân viên" },
                { name: "date", type: "DATE NOT NULL", desc: "Ngày làm thêm giờ" },
                { name: "start_time", type: "TIME NOT NULL", desc: "Giờ bắt đầu OT" },
                { name: "end_time", type: "TIME NOT NULL", desc: "Giờ kết thúc OT" },
                { name: "status", type: "VARCHAR(20) DEFAULT 'pending'", desc: "Trạng thái: pending | approved | rejected" },
                { name: "approved_by", type: "UUID REFERENCES \"user\"(id) ON DELETE SET NULL", desc: "Người duyệt đơn" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Ngày nộp đơn" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 11: audit_log (Nhật ký kiểm toán hệ thống)"),
            createDBTable("audit_log", [
                { name: "id", type: "BIGSERIAL PRIMARY KEY", desc: "Mã log kiểm toán", pk: true },
                { name: "user_id", type: "UUID REFERENCES \"user\"(id) ON DELETE SET NULL", desc: "Mã admin/HR thực hiện thao tác" },
                { name: "action", type: "VARCHAR(100) NOT NULL", desc: "Hành động thực hiện (CREATE_DEVICE...)" },
                { name: "object_type", type: "VARCHAR(50) NOT NULL", desc: "Đối tượng bị tác động (device, employee...)" },
                { name: "object_id", type: "VARCHAR(100)", desc: "ID của đối tượng bị tác động" },
                { name: "description", type: "TEXT", desc: "Mô tả chi tiết nội dung thay đổi" },
                { name: "ip_address", type: "VARCHAR(50)", desc: "Địa chỉ IP của client gọi API" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Thời điểm ghi nhận" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 12: sync_cursor (Lưu con trỏ đồng bộ log chấm công)"),
            createDBTable("sync_cursor", [
                { name: "device_id", type: "UUID PRIMARY KEY REFERENCES device(id) ON DELETE CASCADE", desc: "Mã thiết bị", pk: true },
                { name: "attendance_cursor", type: "TIMESTAMPTZ NOT NULL", desc: "Mốc thời gian log chấm công cuối cùng đã được tải về thành công", pk: false },
                { name: "updated_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Thời điểm cập nhật con trỏ" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 13: employee_device_mapping (Ánh xạ Nhân viên - Thiết bị chấm công)"),
            createDBTable("employee_device_mapping", [
                { name: "id", type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()", desc: "Mã ánh xạ", pk: true },
                { name: "employee_id", type: "UUID REFERENCES employee(id) ON DELETE CASCADE", desc: "Mã nhân viên trong DB" },
                { name: "device_id", type: "UUID REFERENCES device(id) ON DELETE CASCADE", desc: "Mã thiết bị chấm công" },
                { name: "device_user_id", type: "VARCHAR(100) NOT NULL", desc: "Mã định danh của nhân viên trên máy chấm công vật lý" },
                { name: "sync_status", type: "VARCHAR(20) NOT NULL DEFAULT 'pending'", desc: "Trạng thái đồng bộ: pending | success | failed" },
                { name: "fingerprint_enrolled", type: "BOOLEAN DEFAULT false", desc: "Đã có vân tay được ghi nhận ở máy này" },
                { name: "fingerprint_enrolled_at", type: "TIMESTAMPTZ", desc: "Thời điểm quẹt vân tay đăng ký tại máy" },
                { name: "last_synced_at", type: "TIMESTAMPTZ", desc: "Lần đồng bộ tài khoản gần nhất" },
                { name: "last_error", type: "TEXT", desc: "Chi tiết lỗi nếu đồng bộ thất bại" }
            ]),
            createBodyText("Lưu ý: Ràng buộc UNIQUE(employee_id, device_id) và UNIQUE(device_id, device_user_id)."),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 14: attendance_correction (Đơn giải trình sửa giờ công)"),
            createDBTable("attendance_correction", [
                { name: "id", type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()", desc: "Mã đơn giải trình", pk: true },
                { name: "employee_id", type: "UUID REFERENCES employee(id) ON DELETE CASCADE", desc: "Mã nhân viên" },
                { name: "date", type: "DATE NOT NULL", desc: "Ngày cần sửa công" },
                { name: "corrected_time", type: "TIMESTAMPTZ NOT NULL", desc: "Mốc thời gian mong muốn điều chỉnh" },
                { name: "check_type", type: "VARCHAR(10) NOT NULL", desc: "Loại chấm công điều chỉnh: in | out" },
                { name: "reason", type: "TEXT", desc: "Lý do giải trình" },
                { name: "status", type: "VARCHAR(20) DEFAULT 'pending'", desc: "Trạng thái: pending | approved | rejected" },
                { name: "approved_by", type: "UUID REFERENCES \"user\"(id) ON DELETE SET NULL", desc: "Người phê duyệt đơn" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Ngày tạo đơn giải trình" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 15: device_command_queue (Hàng đợi lệnh ADMS Push)"),
            createDBTable("device_command_queue", [
                { name: "id", type: "BIGSERIAL PRIMARY KEY", desc: "Mã hàng đợi tự tăng", pk: true },
                { name: "device_id", type: "UUID REFERENCES device(id) ON DELETE CASCADE", desc: "Mã máy ADMS nhận lệnh" },
                { name: "command_id", type: "BIGINT NOT NULL", desc: "ID lệnh tăng dần dùng để bắt tay ACK" },
                { name: "command", type: "TEXT NOT NULL", desc: "Nội dung lệnh định dạng ADMS (ví dụ: DATA UPDATE USER PIN=1...)" },
                { name: "status", type: "VARCHAR(20) NOT NULL DEFAULT 'pending'", desc: "Trạng thái lệnh: pending | sent | ack | failed" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Thời điểm tạo lệnh" },
                { name: "sent_at", type: "TIMESTAMPTZ", desc: "Thời điểm máy gửi lệnh xuống thiết bị" },
                { name: "acked_at", type: "TIMESTAMPTZ", desc: "Thời điểm thiết bị trả về gói ACK thành công" }
            ]),
            createBodyText("Lưu ý: Ràng buộc UNIQUE(device_id, command_id) để tránh gửi sai thứ tự lệnh."),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 16: employee_fingerprint (Kho lưu vân tay trung tâm)"),
            createDBTable("employee_fingerprint", [
                { name: "id", type: "BIGSERIAL PRIMARY KEY", desc: "Mã vân tay tự tăng", pk: true },
                { name: "employee_id", type: "UUID REFERENCES employee(id) ON DELETE CASCADE", desc: "Mã nhân viên sở hữu" },
                { name: "finger_index", type: "INT NOT NULL", desc: "Số hiệu ngón tay đăng ký (0–9)" },
                { name: "template_data", type: "TEXT NOT NULL", desc: "Chuỗi mẫu vân tay nhị phân mã hóa Base64" },
                { name: "template_size", type: "INT DEFAULT 0", desc: "Kích thước mẫu (bytes)" },
                { name: "algo_version", type: "VARCHAR(20) DEFAULT '10.0'", desc: "Phiên bản thuật toán vân tay: 9.0 | 10.0" },
                { name: "source_device_id", type: "UUID REFERENCES device(id) ON DELETE SET NULL", desc: "Mã thiết bị gốc thực hiện đăng ký" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Ngày tạo" },
                { name: "updated_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Ngày cập nhật" }
            ]),
            createBodyText("Lưu ý: Ràng buộc UNIQUE(employee_id, finger_index) để giới hạn một ngón chỉ có một mẫu."),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 17: role (Phân quyền Vai trò)"),
            createDBTable("role", [
                { name: "id", type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()", desc: "Mã vai trò", pk: true },
                { name: "name", type: "VARCHAR(50) UNIQUE NOT NULL", desc: "Tên vai trò: admin | hr | viewer" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 18: user (Tài khoản Quản trị)"),
            createDBTable("user", [
                { name: "id", type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()", desc: "Mã tài khoản quản trị", pk: true },
                { name: "username", type: "VARCHAR(100) UNIQUE NOT NULL", desc: "Tên đăng nhập hệ thống" },
                { name: "password_hash", type: "VARCHAR(255) NOT NULL", desc: "Mật khẩu mã hóa hash bcrypt" },
                { name: "role_id", type: "UUID REFERENCES role(id)", desc: "Mã vai trò phân quyền" },
                { name: "created_at", type: "TIMESTAMPTZ DEFAULT now()", desc: "Ngày tạo tài khoản" }
            ]),
            new Paragraph({ spacing: { before: 120 } }),

            createHeading3("Bảng 19: permission (Quyền chi tiết)") ,
            createDBTable("permission", [
                { name: "id", type: "UUID PRIMARY KEY DEFAULT gen_random_uuid()", desc: "Mã quyền", pk: true },
                { name: "role_id", type: "UUID REFERENCES role(id) ON DELETE CASCADE", desc: "Mã vai trò áp dụng" },
                { name: "action", type: "VARCHAR(50) NOT NULL", desc: "Hành động: create | read | update | delete" },
                { name: "object", type: "VARCHAR(50) NOT NULL", desc: "Đối tượng: device | employee | attendance_log..." }
            ]),
            createBodyText("Lưu ý: Ràng buộc UNIQUE(role_id, action, object)."),

            new PageBreak(),

            // --- CHƯƠNG 4 ---
            createHeading1("CHƯƠNG 4: ĐẶC TẢ API & TÀI LIỆU POSTMAN"),
            
            createHeading2("4.1. Danh sách chi tiết Swagger API Endpoints"),
            createBodyText("Hệ thống cung cấp hệ thống API chuẩn RESTful được đặc tả bằng OpenAPI v3 tại docs/swagger.yaml. Bảng dưới đây liệt kê toàn bộ các API chính đang hoạt động:"),

            new Table({
                width: { size: 100, type: WidthType.PERCENTAGE },
                rows: [
                    new TableRow({
                        children: [
                            createTableHeaderCell("HTTP Method & Path", 35),
                            createTableHeaderCell("Quyền truy cập", 20),
                            createTableHeaderCell("Mô tả chức năng & Trạng thái phản hồi", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/auth/login", 35, { bold: true }),
                            createTableCell("Public", 20),
                            createTableCell("Xác thực quản trị viên, trả về JWT Token và thông tin user.\n• 200: Thành công\n• 401: Sai mật khẩu/tài khoản", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("GET /api/v1/devices", 35, { bold: true }),
                            createTableCell("Admin, HR, Viewer", 20),
                            createTableCell("Lấy danh sách tất cả máy chấm công trong hệ thống.\n• 200: OK\n• 401: Chưa xác thực", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/devices", 35, { bold: true }),
                            createTableCell("Admin", 20),
                            createTableCell("Khai báo thêm mới một thiết bị chấm công.\n• 201: Created\n• 400: Sai định dạng đầu vào", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/devices/:id/test-connection", 35, { bold: true }),
                            createTableCell("Admin", 20),
                            createTableCell("Yêu cầu server ping và bắt tay thử với phần cứng.\n• 200: Kết nối thành công\n• 500: Thiết bị offline/lỗi kết nối", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/devices/:id/reboot", 35, { bold: true }),
                            createTableCell("Admin", 20),
                            createTableCell("Gửi lệnh reboot khởi động lại thiết bị từ xa.\n• 200: Khởi động thành công\n• 500: Lỗi phần cứng", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/employees", 35, { bold: true }),
                            createTableCell("Admin, HR", 20),
                            createTableCell("Thêm mới hồ sơ nhân sự vào database trung tâm.\n• 201: Created\n• 400: Trùng mã nhân viên", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/employees/sync-to-device", 35, { bold: true }),
                            createTableCell("Admin, HR", 20),
                            createTableCell("Đẩy thông tin tài khoản và thẻ từ nhân viên xuống thiết bị.\n• 200: Đồng bộ thành công\n• 500: Lỗi kết nối thiết bị", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/biometric/enroll", 35, { bold: true }),
                            createTableCell("Admin", 20),
                            createTableCell("Kích hoạt chế độ đăng ký quét vân tay từ xa từ Web UI.\n• 200: Đã gửi lệnh quét\n• 500: Lỗi thiết bị", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/biometric/backup", 35, { bold: true }),
                            createTableCell("Admin", 20),
                            createTableCell("Đọc và sao lưu vân tay của nhân viên từ máy chấm công về database.\n• 200: Sao lưu thành công", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/biometric/push", 35, { bold: true }),
                            createTableCell("Admin", 20),
                            createTableCell("Đẩy mẫu vân tay từ database xuống máy chấm công chỉ định.\n• 200: Phân phối thành công", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/shifts", 35, { bold: true }),
                            createTableCell("Admin, HR", 20),
                            createTableCell("Khai báo ca làm việc mới.\n• 201: Created", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/shifts/assign", 35, { bold: true }),
                            createTableCell("Admin, HR", 20),
                            createTableCell("Lập lịch gán ca làm việc cho danh sách nhân viên.\n• 200: Gán thành công", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/leave/requests", 35, { bold: true }),
                            createTableCell("Admin, HR, Employee", 20),
                            createTableCell("Nộp đơn nghỉ phép (phép năm, phép ốm...).\n• 201: Đã nộp đơn thành công", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/leave/requests/:id/approve", 35, { bold: true }),
                            createTableCell("Admin, HR", 20),
                            createTableCell("Phê duyệt đơn nghỉ phép của nhân sự.\n• 200: Đã duyệt đơn thành công", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/overtime/requests", 35, { bold: true }),
                            createTableCell("Admin, HR, Employee", 20),
                            createTableCell("Đăng ký làm thêm giờ (OT).\n• 201: Created", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/overtime/requests/:id/approve", 35, { bold: true }),
                            createTableCell("Admin, HR", 20),
                            createTableCell("Phê duyệt hoặc Từ chối đơn làm thêm giờ.\n• 200: Đã duyệt", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/attendance/corrections", 35, { bold: true }),
                            createTableCell("Admin, HR, Employee", 20),
                            createTableCell("Nộp đơn giải trình sửa giờ công khi quên quẹt thẻ.\n• 201: Created", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("POST /api/v1/attendance/corrections/:id/approve", 35, { bold: true }),
                            createTableCell("Admin, HR", 20),
                            createTableCell("HR phê duyệt đơn giải trình và tự tạo log quẹt thẻ ảo.\n• 200: Đã duyệt", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("GET /api/v1/reports/monthly", 35, { bold: true }),
                            createTableCell("Admin, HR, Viewer", 20),
                            createTableCell("Xem ma trận bảng công tháng và xuất file Excelize.\n• 200: OK", 45),
                        ],
                    }),
                    new TableRow({
                        children: [
                            createTableCell("GET /api/v1/audit-logs", 35, { bold: true }),
                            createTableCell("Admin", 20),
                            createTableCell("Xem toàn bộ nhật ký thao tác quản trị trên hệ thống.\n• 200: OK", 45),
                        ],
                    }),
                ],
            }),

            createHeading2("4.2. Cơ chế Bảo mật API & Middleware JWT"),
            createBodyText("Toàn bộ các API nghiệp vụ (ngoại trừ API đăng nhập `/api/v1/auth/login` và API nhận log ADMS `/iclock/*`) đều được bảo vệ chặt chẽ thông qua cơ chế Authorization Header dạng Bearer Token:"),
            createBodyText("Authorization: Bearer <JWT_TOKEN>"),
            createBulletPoint("Tầng Middleware trong Go (AuthMiddleware) sẽ chặn mọi request đến, tiến hành bóc tách token."),
            createBulletPoint("Sử dụng thuật toán mã hóa HMAC SHA-256 đối chiếu với jwt_secret khai báo tại file config.yaml để xác minh tính toàn vẹn của token."),
            createBulletPoint("Kiểm tra thời hạn hiệu lực (field exp trong claim của JWT). Nếu token đã hết hạn hoặc không hợp lệ, hệ thống trả về ngay mã lỗi 401 Unauthorized."),
            createBulletPoint("Nếu hợp lệ, giải mã claim để lấy user_id và role, sau đó đẩy thông tin này vào request context (gin.Context) để các handler phía sau sử dụng cho việc phân quyền RBAC và ghi audit log."),

            createHeading2("4.3. Cấu hình Postman Collection & Scripts Tự động hóa"),
            createBodyText("File cấu hình kiểm thử 'docs/postman_collection.json' đi kèm mã nguồn dự án chứa toàn bộ danh sách API được nhóm theo từng module nghiệp vụ tương ứng trên Web UI. Điểm đặc biệt của Collection này là cơ chế tự động hóa quy trình xác thực:"),
            createHeading3("Các bước cấu hình chạy thử:"),
            createBulletPoint("Bước 1: Import file postman_collection.json vào phần mềm Postman."),
            createBulletPoint("Bước 2: Click chuột phải vào Collection -> Chọn Edit -> Ở tab Variables, khai báo biến baseUrl trỏ đến server Go của bạn (ví dụ: http://localhost:8080) và tạo một biến rỗng jwt_token."),
            createBulletPoint("Bước 3: Tại API đăng nhập (POST /api/v1/auth/login), phần Tests đã được tích hợp sẵn đoạn script JavaScript sau:"),
            createBodyText(`var jsonData = pm.response.json();
if (jsonData && jsonData.token) {
    pm.globals.set("jwt_token", jsonData.token);
    console.log("JWT Token has been updated automatically!");
}`),
            createBulletPoint("Bước 4: Nhấn Send API đăng nhập. Script sẽ tự động lấy chuỗi token từ response và gán vào biến toàn cục jwt_token."),
            createBulletPoint("Bước 5: Tất cả các API còn lại trong Collection đều được cấu hình tab Authorization kiểu 'Inherit auth from parent'. Do đó, khi gọi các API này, Postman sẽ tự động đính kèm header Bearer {{jwt_token}} mà người kiểm thử không cần sao chép thủ công."),

            new PageBreak(),

            // --- CHƯƠNG 5 ---
            createHeading1("CHƯƠNG 5: HƯỚNG DẪN TRIỂN KHAI & VẬN HÀNH"),
            
            createHeading2("5.1. Cấu hình chi tiết file config.yaml"),
            createBodyText("Hệ thống nạp toàn bộ cấu hình từ file config.yaml đặt tại thư mục gốc. Quản trị viên hệ thống cần hiểu rõ các tham số sau trước khi vận hành:"),
            createBulletPoint("env: Xác định môi trường chạy (development | production). Môi trường production sẽ tắt chế độ debug của Gin và giảm thiểu log hệ thống thô để tối ưu tốc độ xử lý."),
            createBulletPoint("postgres_dsn: Chuỗi kết nối dạng PostgreSQL DSN. Định nghĩa username, password, host, port và tên database, đồng thời cấu hình pool connection qua pgxpool."),
            createBulletPoint("http_port: Cổng mạng dịch vụ REST API chạy (mặc định: 8080)."),
            createBulletPoint("jwt_secret: Chuỗi khóa bí mật dùng để ký và giải mã JWT token. Cần đổi khóa này trước khi triển khai hệ thống thực tế."),
            createBulletPoint("attendance_sync_cron: Cú pháp cron của scheduler chạy ngầm kéo log chấm công (ví dụ: */15 * * * * nghĩa là định kỳ 15 phút quét log một lần)."),
            createBulletPoint("notifications.email_enabled: Bật/tắt gửi thông báo email. Cấu hình chi tiết SMTP Host (ví dụ: smtp.gmail.com), SMTP Port (587 hoặc 465) và App Password của Gmail."),
            createBulletPoint("notifications.zalo_enabled: Bật/tắt gửi tin nhắn Zalo. Cấu hình zalo_api_url và zalo_access_token nhận từ trang Zalo Cloud Account để gọi API gửi tin nhắn."),
            createBulletPoint("notifications.checkout_grace_minutes: Số phút ân hạn sau giờ kết thúc ca. Nếu quá thời gian này nhân viên chưa check-out, hệ thống sẽ kích hoạt worker nhắc nhở gửi tin nhắn cảnh báo."),

            createHeading2("5.2. Hướng dẫn chi tiết cài đặt và cấu hình COM SDK trên Windows"),
            createBodyText("Đối với các thiết bị chấm công ZKTeco Standalone (sử dụng zkemkeeper.dll), server backend phải chạy trên môi trường Windows (Server hoặc Desktop) và đăng ký COM ActiveX thành công theo các bước nghiêm ngặt sau:"),
            createBulletPoint("Bước 1: Giải nén gói cài đặt SDK (chứa zkemkeeper.dll và commxtest.dll cùng các file phụ thuộc). Sao chép toàn bộ các file này vào thư mục C:\\Windows\\System32 (nếu dùng Windows 32-bit) hoặc C:\\Windows\\SysWOW64 (nếu dùng Windows 64-bit)."),
            createBulletPoint("Bước 2: Click nút Start -> Tìm kiếm cmd hoặc PowerShell -> Click chuột phải chọn 'Run as Administrator'."),
            createBulletPoint("Bước 3: Điều hướng tới thư mục chứa file dll và chạy lệnh đăng ký: regsvr32.exe zkemkeeper.dll. Hệ thống sẽ hiển thị hộp thoại thông báo đăng ký thành công (RegSvr32: DllRegisterServer in zkemkeeper.dll succeeded)."),
            createBulletPoint("Bước 4: Thiết lập phân quyền cấu hình COM (DCOMCNFG): Chạy lệnh dcomcnfg.exe trong Windows -> Component Services -> Computers -> My Computer -> DCOM Config -> Tìm ZKEMKeeper -> Click chuột phải chọn Properties -> Ở tab Security, cấp quyền Launch and Activation Permissions và Access Permissions cho User chạy backend Go."),

            createHeading2("5.3. Khởi chạy hệ thống bằng Docker Compose"),
            createBodyText("Hệ thống hỗ trợ triển khai nhanh chóng thông qua Docker Compose cho môi trường database PostgreSQL:"),
            createBulletPoint("Bước 1: Kiểm tra tệp docker-compose.yml ở thư mục gốc. Nó định nghĩa dịch vụ postgres sử dụng image postgres:15-alpine, mở cổng 5432 và lưu trữ dữ liệu tại volume pgdata."),
            createBulletPoint("Bước 2: Mở terminal tại thư mục dự án và khởi chạy cơ sở dữ liệu: docker-compose up -d."),
            createBulletPoint("Bước 3: Docker sẽ tự động tải image, cấu hình container và chạy các script khởi tạo DB (migration). Bạn có thể kiểm tra trạng thái hoạt động bằng lệnh: docker-compose ps."),
            createBulletPoint("Bước 4: Biên dịch ứng dụng Go: go build -o server.exe cmd/server/main.go."),
            createBulletPoint("Bước 5: Khởi chạy file thực thi server.exe (trên Windows) hoặc chạy trực tiếp bằng lệnh: go run cmd/server/main.go."),

            new PageBreak(),

            // --- CHƯƠNG 6 ---
            createHeading1("CHƯƠNG 6: BÁO CÁO KIỂM THỬ (TEST REPORT)"),
            
            createHeading2("6.1. Thiết lập Kiểm thử Đơn vị (Unit Tests)"),
            createBodyText("Để kiểm soát lỗi phát sinh trong quá trình cập nhật mã nguồn, dự án triển khai hệ thống Unit Test bao phủ các tầng logic nghiệp vụ chính. Tận dụng cơ chế interface trong Clean Architecture, chúng tôi sử dụng Mocking để giả lập database và adapter vật lý của thiết bị:"),
            createBulletPoint("Framework kiểm thử: Sử dụng thư viện test chuẩn của Go kết hợp package stretchr/testify để viết mã ngắn gọn và trực quan."),
            createBulletPoint("Tệp tin mock: test/mocks.go triển khai giả lập các interface repository (DeviceRepository, EmployeeRepository, AttendanceLogRepository, ShiftRepository...) trả về dữ liệu giả lập có sẵn."),
            createBulletPoint("Lệnh chạy toàn bộ hệ thống test: go test ./internal/usecase/... -v -cover"),
            createBulletPoint("Độ bao phủ thực tế (Coverage): Tầng Usecase đạt mức trung bình 88.9%, đảm bảo mọi giải thuật phân ca, tính toán đi trễ, về sớm, bù phép đều được kiểm thử kỹ lưỡng trước khi đưa vào sản xuất."),

            createHeading2("6.2. Kịch bản Kiểm thử Tích hợp chi tiết (Integration Test Scenarios)"),
            createBodyText("Dưới đây là 4 kịch bản kiểm thử tích hợp thực tế được bộ phận IT và HR phối hợp chạy thử nghiệm trên môi trường có thiết bị vật lý kết nối:"),

            createHeading3("Kịch bản 1: Đồng bộ vân tay và đăng ký ngón tay từ xa (Remote Enroll)"),
            createBulletPoint("• Mô tả: Người quản trị kích hoạt chế độ đăng ký vân tay mới của nhân viên Nguyễn Văn A (Mã số: 102) từ xa thông qua màn hình Web UI."),
            createBulletPoint("• Các bước thực hiện:\n1. Admin truy cập màn hình hồ sơ nhân viên Nguyễn Văn A, click chọn ngón tay số 0 (ngón cái phải) và chọn máy chấm công ZKTeco Văn Phòng.\n2. Click nút 'Kích hoạt đăng ký từ xa'.\n3. Nguyễn Văn A đứng trước máy chấm công vật lý. Khi máy chấm công phát tiếng bíp và màn hình hiển thị yêu cầu quét ngón tay, thực hiện đặt ngón tay 3 lần liên tục lên mắt đọc."),
            createBulletPoint("• Kết quả mong đợi:\n1. Máy chấm công thông báo đăng ký thành công.\n2. Server nhận log đăng ký, tự động tải template vân tay về DB lưu trữ tại bảng employee_fingerprint dưới dạng Base64 và cập nhật trạng thái fingerprint_enrolled của nhân viên thành true trên Web UI."),

            createHeading3("Kịch bản 2: Đồng bộ nhân sự và đẩy tài khoản xuống thiết bị (Push to Devices)"),
            createBulletPoint("• Mô tả: HR tạo hồ sơ nhân viên mới và yêu cầu đẩy thông tin tài khoản (mã số, họ tên, thẻ từ) xuống tất cả các máy chấm công."),
            createBulletPoint("• Các bước thực hiện:\n1. HR vào Web UI, click 'Thêm nhân viên' -> nhập tên Trần Thị B, mã số 105, mã thẻ từ 88776655.\n2. Click nút 'Đồng bộ xuống tất cả thiết bị'.\n3. Đi tới 3 máy chấm công thực tế (ZKTeco, Hikvision, Sunbeam) để quét thử thẻ từ 88776655 lên đầu đọc thẻ."),
            createBulletPoint("• Kết quả mong đợi: Cả 3 máy chấm công đều nhận diện được thẻ từ của Trần Thị B, hiển thị tên 'Tran Thi B' trên màn hình LCD và phát ra lời chào xác nhận quẹt thẻ thành công."),

            createHeading3("Kịch bản 3: Xử lý Log chấm công ca đêm qua ngày (Overnight Shift)"),
            createBulletPoint("• Mô tả: Kiểm thử nghiệp vụ gom ca làm việc kéo dài qua ngày hôm sau của nhân viên ca đêm."),
            createBulletPoint("• Các bước thực hiện:\n1. Phân ca cho nhân viên Nguyễn Văn A làm ca đêm (Ca từ 22:00 hôm trước đến 06:00 sáng hôm sau).\n2. Nhân viên quét vân tay check-in lúc 21:45 tối ngày 19/07.\n3. Nhân viên quét vân tay check-out lúc 06:15 sáng ngày 20/07.\n4. Chạy tác vụ tổng hợp công ngày 19/07."),
            createBulletPoint("• Kết quả mong đợi: Bảng công ngày 19/07 của Nguyễn Văn A hiển thị đầy đủ giờ Check-In: 21:45, Check-Out: 06:15, số giờ làm việc quy đổi đạt 8.0 giờ, trạng thái công là 'present'. Bảng công ngày 20/07 không bị phát sinh log đi trễ/về sớm bất thường."),

            createHeading3("Kịch bản 4: Cảnh báo quên Check-Out tự động (Checkout Reminder Monitor)"),
            createBulletPoint("• Mô tả: Kiểm thử tính năng nhắc nhở chấm công tự động thông qua Worker chạy nền."),
            createBulletPoint("• Các bước thực hiện:\n1. Nhân viên Trần Thị B làm ca hành chính (kết thúc ca lúc 17:30).\n2. Trần Thị B check-in lúc 08:00 sáng nhưng đến 18:00 chiều vẫn không thực hiện check-out.\n3. Chờ worker kiểm tra (chạy định kỳ mỗi phút)."),
            createBulletPoint("• Kết quả mong đợi: Đúng 18:00 (quá giờ kết thúc ca 30 phút), hệ thống phát hiện thiếu log check-out, tự động kích hoạt SMTP gửi email cảnh báo về hòm thư của Trần Thị B, đồng thời gọi Zalo OA API gửi tin nhắn nhắc nhở quẹt thẻ bổ sung trực tiếp về số điện thoại đăng ký."),

            new PageBreak(),

            // --- CHƯƠNG 7 ---
            createHeading1("CHƯƠNG 7: ĐẶC TẢ CHI TIẾT TÍNH NĂNG TRÊN GIAO DIỆN NGƯỜI DÙNG (GUI)"),

            createHeading2("7.1. Phân hệ Tổng quan (Dashboard)"),
            createBodyText("Phân hệ Tổng quan là trung tâm giám sát hoạt động thời gian thực của hệ thống, giúp người quản trị có cái nhìn toàn cảnh về tình trạng nhân sự và thiết bị phần cứng:"),
            createBulletPoint("Thẻ chỉ số thống kê (Stat Cards): Hiển thị trực quan 4 chỉ số cốt lõi gồm: Tổng số thiết bị đã khai báo (kèm số lượng online/offline), Tổng số nhân sự hiện hữu, Số lượng bản ghi chấm công thô thu thập trong ngày hiện tại, và Tổng số phiên đồng bộ đã thực hiện."),
            createBulletPoint("Biểu đồ phân tích chuyên cần hôm nay (Doughnut Chart): Vẽ biểu đồ tròn thể hiện tỷ lệ phần trăm các trạng thái nhân sự trong ngày: Đi làm đúng giờ (Present), Đi muộn (Late), Vắng mặt (Absent), và Nghỉ phép (Leave) giúp bộ phận HR nắm bắt nhanh quân số."),
            createBulletPoint("Biểu đồ lịch sử chấm công 7 ngày (Bar Chart): Thống kê tổng số lượng lượt quét thẻ/vân tay thành công qua từng ngày trong tuần, giúp đánh giá tần suất hoạt động và phát hiện các ngày biến động bất thường."),
            createBulletPoint("Bảng giám sát quét thời gian thực (Real-time Last Scan Card): Tích hợp giao thức Server-Sent Events (SSE) kết nối trực tiếp từ Go server. Ngay khi nhân viên quét vân tay/khuôn mặt trên bất kỳ máy chấm công nào, thông tin chi tiết lập tức hiển thị trên màn hình admin mà không cần tải lại trang. Thông tin bao gồm: Ảnh đại diện nhân viên, Họ tên, Mã nhân viên, Phòng ban, Mốc thời gian quẹt chính xác, Loại quét (Vào/Ra), Phương thức xác thực (Vân tay/Thẻ/Khuôn mặt) và Tên thiết bị ghi nhận."),
            createBulletPoint("Danh sách hiển thị nhanh: Liệt kê các thiết bị kết nối gần nhất và lịch sử đồng bộ dữ liệu mới nhất."),

            createHeading2("7.2. Phân hệ Quản lý Thiết bị (Devices)"),
            createBodyText("Phân hệ này cung cấp giao diện quản trị vòng đời của các thiết bị chấm công vật lý trong toàn doanh nghiệp:"),
            createBulletPoint("Bảng danh sách thiết bị: Liệt kê tất cả các máy chấm công kèm theo thông tin chi tiết: Tên thiết bị, Hãng sản xuất/Phiên bản Firmware, Địa chỉ IP/Cổng kết nối, Trạng thái kết nối (Online/Offline) được cập nhật tự động và Thời gian hoạt động gần nhất."),
            createBulletPoint("Khai báo thiết bị: Form thêm mới và chỉnh sửa thiết bị hỗ trợ đầy đủ các tham số: Tên máy, Hãng máy (zkteco, hikvision, sunbeam), Địa chỉ IP tĩnh, Cổng kết nối, Số Serial (SDK Pull), Số Serial ADMS (Push), Bật/Tắt chế độ ADMS Push, Vị trí đặt máy, và thông tin xác thực API (Username, Password) dùng cho các thiết bị Hikvision ISAPI."),
            createBulletPoint("Kiểm tra kết nối (Test Connection): Cho phép admin bấm nút kiểm tra nhanh trạng thái vật lý của máy chấm công qua mạng LAN/WAN. Hệ thống sẽ thực hiện bắt tay thử và hiển thị toast thông báo kết quả lập tức."),
            createBulletPoint("Khởi động lại từ xa (Reboot Device): Gửi lệnh điều khiển tắt và khởi động lại thiết bị trực tiếp từ giao diện Web quản trị mà không cần thao tác vật lý trên máy."),
            createBulletPoint("Đồng bộ & Dọn dẹp: Hỗ trợ dọn dẹp hàng đợi lệnh chờ xử lý (Clear Command Queue) đối với các máy ADMS Push."),

            createHeading2("7.3. Phân hệ Quản lý Nhân sự (Employees)"),
            createBodyText("Phân hệ quản lý hồ sơ nhân viên tập trung và điều phối phân phối sinh trắc học xuống các đầu đọc phần cứng:"),
            createBulletPoint("Danh sách nhân viên chuyên nghiệp: Hiển thị bảng lưới nhân sự, hỗ trợ tìm kiếm nhanh theo Họ tên, Mã nhân viên hoặc bộ phận trực thuộc. Bảng hiển thị rõ trạng thái sinh trắc học (Đã đăng ký thẻ từ/vân tay hay chưa) và trạng thái làm việc (Active/Inactive)."),
            createBulletPoint("Hồ sơ nhân viên chi tiết: Cho phép thêm mới và chỉnh sửa hồ sơ nhân viên gồm: Mã NV, Họ tên, Chức danh, Phòng ban (HR, IT, Sales, Marketing, Kế toán, R&D, Sản xuất, CS), Số thẻ từ, Email nhận thông báo, Số điện thoại, Giới tính, Ngày sinh, Ngày vào làm, URL ảnh đại diện và Trạng thái nhân sự."),
            createBulletPoint("Import Excel hàng loạt: Hỗ trợ HR nhập danh sách hàng loạt nhân viên từ file Excel (.xlsx) theo biểu mẫu chuẩn giúp tiết kiệm thời gian nhập liệu thủ công."),
            createBulletPoint("Tác vụ quản trị hàng loạt:"),
            createBulletPoint("  - Đẩy tất cả nhân viên active xuống thiết bị chấm công (Push All To Device) để phân phối tài khoản hàng loạt."),
            createBulletPoint("  - Kéo nhân sự từ thiết bị về web (Pull From Device): Tự động tải danh sách tài khoản đăng ký trực tiếp trên máy chấm công về database."),
            createBulletPoint("  - Xóa toàn bộ nhân viên khỏi hệ thống (Delete All Employees) kèm theo cảnh báo xác nhận nhiều lớp để dọn dẹp dữ liệu."),
            createBulletPoint("Đăng ký vân tay từ xa (Remote Enroll): Quản trị viên chọn nhân viên, chọn ngón tay cần đăng ký (từ 0 đến 9), và chọn máy chấm công. Khi nhấn bắt đầu, máy chấm công sẽ chuyển sang chế độ đăng ký, nhân viên đặt ngón tay 3 lần. Server tự động lưu trữ template Base64 vào database trung tâm."),
            createBulletPoint("Sao chép & Đồng bộ Thiết bị (Backup & Sync Device): Hỗ trợ chọn thiết bị nguồn và thiết bị đích trống, sau đó sao chép toàn bộ danh sách nhân viên và mẫu vân tay đã lưu trữ sang thiết bị đích một cách tự động và an toàn."),

            createHeading2("7.4. Phân hệ Quản lý Ca & Gán ca (Shifts & Assignment)"),
            createBodyText("Phân hệ thiết lập lịch làm việc và gán ca để làm cơ sở cho engine tính toán công ngày:"),
            createBulletPoint("Khai báo ca làm việc linh hoạt: Cho phép tạo mới các ca làm việc với các thông số: Tên ca (Ca Hành chính, Ca Sáng, Ca Chiều, Ca Đêm...), Giờ bắt đầu ca, Giờ kết thúc ca, Thời gian nghỉ giải lao (phút), Số phút đi muộn cho phép (Late Grace), Số phút về sớm cho phép (Early Grace) và Mã màu sắc (Color Code) để hiển thị lịch."),
            createBulletPoint("Gán ca làm việc (Assign Shift): Cho phép gán ca làm việc cho nhân viên theo chu kỳ thời gian (Từ ngày... Đến ngày...). Hệ thống sẽ lưu vết lịch sử gán ca của từng nhân viên."),
            createBulletPoint("Tính công tự động (Process Attendance): Giao diện cho phép HR chọn một ngày cụ thể và bấm nút 'Tính công'. Engine backend sẽ chạy quy trình đối chiếu toàn bộ log thô của ngày đó với lịch gán ca để tính ra giờ vào/ra đầu cuối, số phút trễ/sớm và tổng giờ công thực tế."),

            createHeading2("7.5. Phân hệ Đơn từ & Tự phục vụ (ESS - Employee Self Service)"),
            createBodyText("Cung cấp cổng thông tin tự phục vụ cho người lao động để giảm tải cho bộ phận nhân sự và minh bạch hóa dữ liệu:"),
            createBulletPoint("Nộp đơn đăng ký: Người lao động hoặc HR đại diện gửi các loại đơn từ gồm:"),
            createBulletPoint("  - Đơn nghỉ phép (Leave Request): Chọn loại phép (nghỉ phép năm, nghỉ ốm, thai sản, không lương), khoảng ngày nghỉ và lý do giải trình."),
            createBulletPoint("  - Đơn làm thêm giờ (Overtime Request): Chọn ngày làm thêm, mốc giờ bắt đầu/kết thúc làm thêm và lý do tăng ca."),
            createBulletPoint("  - Đơn giải trình sửa giờ công (Attendance Correction): Cho trường hợp quên quẹt thẻ/vân tay, chọn ngày cần sửa, loại check (Vào/Ra), mốc giờ thực tế muốn ghi nhận và lý do giải trình."),
            createBulletPoint("Quản lý & Phê duyệt đơn (Approval System): Giao diện lọc đơn theo loại và trạng thái (Chờ duyệt, Đã duyệt, Từ chối). HR/Admin có quyền xem xét lý do, bấm duyệt (Approve) đơn để hệ thống tự động cập nhật công (đối với đơn sửa công, hệ thống tự động sinh log ảo chấm công tương ứng) hoặc từ chối (Reject) đơn."),

            createHeading2("7.6. Phân hệ Dữ liệu Chấm công (Attendance Data)"),
            createBodyText("Giao diện tra cứu dữ liệu chấm công chi tiết của nhân sự, hỗ trợ lọc theo mốc thời gian và mã nhân viên:"),
            createBulletPoint("Tóm tắt công ngày (Daily Attendance Tab): Hiển thị bảng tổng hợp công sau khi tính toán gồm: Tên nhân viên, Ngày làm việc, Ca áp dụng, Giờ Vào đầu tiên (First In), Giờ Ra cuối cùng (Last Out), Số phút đi muộn, Số phút về sớm, Số giờ làm việc thực tế, Số giờ tăng ca (OT), Ngày nghỉ phép liên đới và Trạng thái công ngày (Đi làm, Muộn, Vắng, Phép)."),
            createBulletPoint("Nhật ký log thô (Raw Logs Tab): Hiển thị danh sách tất cả các bản ghi chấm công thô thu thập từ các máy chấm công gồm: Thời gian check-time chính xác, Tên/Mã nhân viên, Loại check (Vào/Ra), Phương thức xác thực và Máy chấm công ghi nhận."),
            createBulletPoint("Tính năng kiểm soát giờ quét ±1 tiếng (Validity Check): Cột 'Kiểm tra Giờ' trên bảng hiển thị trạng thái hợp lệ của mỗi lần quét:"),
            createBulletPoint("  - Nhãn 'Trong giờ' (Valid) màu xanh: Log quẹt nằm trong khoảng thời gian làm việc ca gán ±1 tiếng."),
            createBulletPoint("  - Nhãn 'Ngoài giờ' (Invalid) màu đỏ: Log quẹt nằm ngoài khoảng thời gian làm việc ca gán ±1 tiếng (lý do: Outside shift window +-1h) hoặc nhân viên chưa được gán ca làm việc (lý do: No shift assigned). Toàn bộ log ngoài giờ này sẽ không được engine tính công tính vào giờ vào đầu/ra cuối."),

            createHeading2("7.7. Phân hệ Báo cáo Tháng (Monthly Reports)"),
            createBodyText("Phân hệ tổng hợp bảng công tháng và xuất file báo cáo phục vụ tính lương:"),
            createBulletPoint("Bảng ma trận công tháng: Hiển thị danh sách nhân viên ở các hàng, các ngày trong tháng ở các cột. Mỗi ô ngày hiển thị ký hiệu trạng thái công (P: Đi làm đúng giờ, L: Đi muộn, A: Vắng mặt, V: Nghỉ phép)."),
            createBulletPoint("Tooltip tương tác: Khi di chuột (hover) vào từng ô ngày, hệ thống hiển thị chi tiết giờ Vào/Ra, số giờ làm việc, đi trễ, về sớm, và thống kê số lần quét vân tay hợp lệ / không hợp lệ của ngày đó."),
            createBulletPoint("Thống kê quét vân tay tháng: Bảng báo cáo tích hợp 2 cột tổng kết ở cuối bảng:"),
            createBulletPoint("  - Quét hợp lệ (Valid Scans): Tổng số lần chấm công hợp lệ trong tháng."),
            createBulletPoint("  - Không hợp lệ (Invalid Scans): Tổng số lần chấm công không hợp lệ (ngoài khung giờ hoặc không gán ca)."),
            createBulletPoint("Xuất Excel (CSV): Cho phép tải file báo cáo chấm công tháng chi tiết về máy tính để bộ phận kế toán tính lương."),
            createBulletPoint("Gửi báo cáo qua Gmail: HR bấm nút gửi báo cáo tháng, hệ thống tự động tạo file CSV bảng công chi tiết cá nhân và gửi email trực tiếp tới hòm thư của từng nhân viên."),

            createHeading2("7.8. Phân hệ Nhật ký Hệ thống & Đồng bộ (Audit Logs & Sync History)"),
            createBodyText("Phân hệ ghi nhận dấu vết hệ thống phục vụ mục tiêu kiểm toán bảo mật và giám sát đồng bộ:"),
            createBulletPoint("Nhật ký hoạt động (Audit Logs): Bảng ghi chi tiết các hành động quản trị hệ thống: Thời gian thực hiện, Tài khoản thực hiện, Loại hành động (CREATE, UPDATE, DELETE thiết bị/nhân viên), Mô tả chi tiết nội dung thay đổi và Địa chỉ IP của client gọi API."),
            createBulletPoint("Lịch sử đồng bộ (Sync History): Thống kê chi tiết các phiên đồng bộ dữ liệu: Thiết bị thực hiện, Loại đồng bộ (nhân sự, chấm công), Hình thức kích hoạt (Thủ công/Tự động), Trạng thái (Thành công/Thất bại), Số bản ghi xử lý thành công, Thời gian và thông báo lỗi chi tiết nếu có."),
            createBulletPoint("Đồng bộ ngay (Sync Now): Nút bấm ép buộc hệ thống chạy ngầm quét và tải log chấm công tức thời từ toàn bộ thiết bị đang online."),

            createHeading2("7.9. Phân hệ Cấu hình hệ thống (Settings)"),
            createBodyText("Hiển thị các thông tin cấu hình cơ bản của hệ thống và bảng tài liệu hướng dẫn nhanh từng bước vận hành các chức năng cốt lõi cho quản trị viên mới."),

            new Paragraph({ spacing: { before: 800 } }),
            new Paragraph({
                alignment: AlignmentType.CENTER,
                children: [
                    new TextRun({
                        text: "--- HẾT TÀI LIỆU BÀN GIAO ---",
                        bold: true,
                        size: 20,
                        font: "Arial",
                        color: COLOR_SECONDARY,
                    }),
                ],
            }),
        ],
    }],
});

Packer.toBuffer(doc).then((buffer) => {
    const destPath = path.join("e:", "Project", "attendance-system", "TAI_LIEU_BAN_GIAO_KY_THUAT_V3.docx");
    fs.writeFileSync(destPath, buffer);
    console.log("Successfully generated technical document aligned with codebase features at:", destPath);
}).catch((err) => {
    console.error("Failed to generate document:", err);
    process.exit(1);
});
