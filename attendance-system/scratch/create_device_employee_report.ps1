$ErrorActionPreference = 'Stop'

$outPath = Join-Path (Get-Location) 'BAO_CAO_CHUC_NANG_THIET_BI_VA_NHAN_VIEN.docx'
$word = $null
$doc = $null

function Set-DocumentFont {
    param([object]$Document)
    foreach ($styleName in @('Normal', 'Title', 'Subtitle', 'Heading 1', 'Heading 2', 'Heading 3', 'List Bullet', 'List Number')) {
        try {
            $style = $Document.Styles.Item($styleName)
            $style.Font.Name = 'Arial'
            $style.Font.NameFarEast = 'Arial'
            $style.Font.Size = 10.5
        } catch {
            # Một số bản Word không có đầy đủ style theo tên; nội dung vẫn được định dạng ở cấp đoạn.
        }
    }
}

function Add-Paragraph {
    param(
        [Parameter(Mandatory = $true)][string]$Text,
        [string]$Style = 'Normal',
        [int]$Size = 10,
        [switch]$Bold,
        [int]$Alignment = 0,
        [int]$SpaceAfter = 6
    )
    $p = $script:doc.Paragraphs.Add()
    try { $p.Style = $Style } catch { }
    $p.Range.Text = $Text
    $p.Range.Font.Name = 'Arial'
    $p.Range.Font.NameFarEast = 'Arial'
    $p.Range.Font.Size = $Size
    $p.Range.Font.Bold = if ($Bold) { 1 } else { 0 }
    $p.Range.ParagraphFormat.Alignment = $Alignment
    $p.Range.ParagraphFormat.SpaceAfter = $SpaceAfter
    $p.Range.ParagraphFormat.LineSpacingRule = 0
    $p.Range.InsertParagraphAfter()
    return $p
}

function Add-Heading {
    param([string]$Text, [int]$Level = 1)
    $style = if ($Level -eq 1) { 'Heading 1' } elseif ($Level -eq 2) { 'Heading 2' } else { 'Heading 3' }
    $size = if ($Level -eq 1) { 15 } elseif ($Level -eq 2) { 12 } else { 11 }
    $p = Add-Paragraph -Text $Text -Style $style -Size $size -Bold -SpaceAfter 5
    $p.Range.Font.Color = if ($Level -eq 1) { 0x1F4E79 } elseif ($Level -eq 2) { 0x2F5597 } else { 0x404040 }
    return $p
}

function Add-BulletList {
    param([string[]]$Items)
    foreach ($item in $Items) {
        Add-Paragraph -Text $item -Style 'List Bullet' -Size 10 -SpaceAfter 2 | Out-Null
    }
}

function Add-NumberList {
    param([string[]]$Items)
    foreach ($item in $Items) {
        Add-Paragraph -Text $item -Style 'List Number' -Size 10 -SpaceAfter 2 | Out-Null
    }
}

function Add-Table {
    param(
        [Parameter(Mandatory = $true)][object[][]]$Rows,
        [int]$FontSize = 9,
        [int]$HeaderColor = 15130860
    )
    if ($Rows.Count -eq 0) { return }
    $columns = $Rows[0].Count
    $range = $script:doc.Range($script:doc.Content.End - 1, $script:doc.Content.End - 1)
    $table = $script:doc.Tables.Add($range, $Rows.Count, $columns)
    try { $table.Style = 'Table Grid' } catch { }
    $table.AllowAutoFit = $true
    $table.AutoFitBehavior(1) | Out-Null

    for ($r = 0; $r -lt $Rows.Count; $r++) {
        for ($c = 0; $c -lt $columns; $c++) {
            $cell = $table.Cell($r + 1, $c + 1)
            $value = if ($c -lt $Rows[$r].Count) { [string]$Rows[$r][$c] } else { '' }
            $cell.Range.Text = $value
            $cell.Range.Font.Name = 'Arial'
            $cell.Range.Font.NameFarEast = 'Arial'
            $cell.Range.Font.Size = $FontSize
            $cell.Range.ParagraphFormat.SpaceAfter = 1
            if ($r -eq 0) {
                $cell.Range.Font.Bold = 1
                try { $cell.Shading.BackgroundPatternColor = $HeaderColor } catch { }
            }
        }
    }
    $after = $script:doc.Range($script:doc.Content.End - 1, $script:doc.Content.End - 1)
    $after.InsertParagraphAfter()
    return $table
}

function Add-PageBreak {
    $range = $script:doc.Range($script:doc.Content.End - 1, $script:doc.Content.End - 1)
    $range.InsertBreak(7) # wdPageBreak
}

function Add-FieldFooter {
    foreach ($section in $script:doc.Sections) {
        $footer = $section.Footers.Item(1)
        $footer.Range.Text = 'Báo cáo chức năng Thiết bị và Nhân viên | Hệ thống quản lý chấm công | 17/07/2026'
        $footer.Range.Font.Name = 'Arial'
        $footer.Range.Font.Size = 8
        $footer.Range.ParagraphFormat.Alignment = 2
    }
}

try {
    $word = New-Object -ComObject Word.Application
    $word.Visible = $false
    $word.DisplayAlerts = 0
    $doc = $word.Documents.Add()
    $script:doc = $doc

    $doc.PageSetup.Orientation = 0
    $doc.PageSetup.TopMargin = $word.CentimetersToPoints(2.0)
    $doc.PageSetup.BottomMargin = $word.CentimetersToPoints(1.8)
    $doc.PageSetup.LeftMargin = $word.CentimetersToPoints(2.2)
    $doc.PageSetup.RightMargin = $word.CentimetersToPoints(1.8)
    Set-DocumentFont -Document $doc

    # Trang bìa
    Add-Paragraph -Text 'BÁO CÁO CHỨC NĂNG' -Style 'Title' -Size 24 -Bold -Alignment 1 -SpaceAfter 8 | Out-Null
    $title = Add-Paragraph -Text 'PHÂN HỆ THIẾT BỊ VÀ NHÂN VIÊN' -Style 'Title' -Size 20 -Bold -Alignment 1 -SpaceAfter 22
    $title.Range.Font.Color = 0x1F4E79
    Add-Paragraph -Text 'Hệ thống quản lý chấm công và sinh trắc học' -Style 'Subtitle' -Size 13 -Alignment 1 -SpaceAfter 36 | Out-Null
    Add-Paragraph -Text 'Báo cáo mô tả chức năng, luồng xử lý, giao thức kết nối và điều kiện vận hành' -Size 11 -Alignment 1 -SpaceAfter 50 | Out-Null
    Add-Paragraph -Text 'Ngày lập: 17/07/2026' -Size 11 -Alignment 1 -SpaceAfter 5 | Out-Null
    Add-Paragraph -Text 'Phạm vi: mục Thiết bị và mục Nhân viên' -Size 11 -Alignment 1 -SpaceAfter 5 | Out-Null
    Add-Paragraph -Text 'Căn cứ: giao diện web và mã xử lý hiện hành của dự án' -Size 10 -Alignment 1 -SpaceAfter 5 | Out-Null
    Add-PageBreak

    # Mục lục thủ công để tài liệu mở lên là đọc được ngay.
    Add-Heading -Text 'MỤC LỤC' -Level 1 | Out-Null
    Add-NumberList -Items @(
        'Tóm tắt điều hành và phạm vi báo cáo',
        'Phân hệ Thiết bị',
        'Phân hệ Nhân viên',
        'Luồng đồng bộ dữ liệu và vân tay',
        'Phân quyền, nhật ký và yêu cầu vận hành',
        'Checklist kiểm tra nghiệm thu',
        'Kết luận và đề xuất sử dụng'
    )
    Add-Paragraph -Text 'Ghi chú đọc báo cáo: các chức năng được đánh dấu “đang hiển thị” là nút/luồng có trên giao diện hiện hành; các chức năng “API nền” vẫn có trong server nhưng có thể đang được ẩn khỏi bảng thao tác để tránh thao tác nhầm.' -Size 9 -SpaceAfter 12 | Out-Null

    Add-Heading -Text '1. TÓM TẮT ĐIỀU HÀNH VÀ PHẠM VI BÁO CÁO' -Level 1 | Out-Null
    Add-Paragraph -Text 'Hệ thống quản lý chấm công kết nối máy chấm công, quản lý hồ sơ nhân viên, đồng bộ người dùng hai chiều và quản lý mẫu vân tay. Báo cáo này tập trung vào hai phân hệ mà người quản trị thao tác thường xuyên: Thiết bị và Nhân viên.' -Size 10.5 | Out-Null
    Add-Paragraph -Text 'Kiến trúc xử lý được tổ chức theo các lớp HTTP → usecase → adapter thiết bị → cơ sở dữ liệu. Tùy loại máy, hệ thống dùng kết nối SDK/PULL trực tiếp hoặc ADMS Push qua hàng đợi lệnh. Dữ liệu chấm công thô được tiếp nhận và lưu để phục vụ các màn hình báo cáo khác; hệ thống không tự thay thế phần mềm tính lương.' -Size 10.5 | Out-Null
    Add-Heading -Text '1.1. Thuật ngữ chính' -Level 2 | Out-Null
    Add-Table -Rows @(
        @('Thuật ngữ', 'Ý nghĩa trong hệ thống'),
        @('SDK/PULL', 'Server kết nối trực tiếp đến thiết bị (đặc biệt ZKTeco qua COM SDK trên Windows) để đọc/ghi người dùng, vân tay và log.'),
        @('ADMS Push', 'Thiết bị chủ động gọi endpoint hàng đợi, thường là /iclock/getrequest, để nhận lệnh và gửi dữ liệu về server.'),
        @('Mapping', 'Liên kết giữa nhân viên trên web và mã/PIN người dùng trên từng thiết bị.'),
        @('Template vân tay', 'Mẫu sinh trắc học đã mã hóa; database hỗ trợ finger_index từ 0 đến 9 cho mỗi nhân viên.'),
        @('Hàng đợi lệnh', 'Các lệnh USER/FINGERTEMPLATE hoặc lệnh quét được lưu chờ thiết bị nhận ở lần polling tiếp theo.')
    ) | Out-Null
    Add-Heading -Text '1.2. Phạm vi và cách đọc' -Level 2 | Out-Null
    Add-BulletList -Items @(
        'Báo cáo mô tả hành vi hiện có trong web/index.html, web/app.js và các HTTP handler/usecase tương ứng.',
        'Các thao tác tạo, sửa, xóa, import, đồng bộ, quét và reset có kiểm soát quyền quản trị ở tầng route.',
        'Luồng vân tay được tách thành đăng ký (Web → Máy), kéo/đọc (Máy → Web), lưu database và đẩy sang máy khác.',
        'Thông tin “tối đa 10 ngón” nghĩa là hệ thống có thể lưu/đọc 10 vị trí; thao tác đăng ký người dùng hiện hành vẫn thực hiện từng lượt một ngón.'
    )

    # Thiết bị
    Add-PageBreak
    Add-Heading -Text '2. PHÂN HỆ THIẾT BỊ' -Level 1 | Out-Null
    Add-Paragraph -Text 'Phân hệ Thiết bị là nơi khai báo điểm kết nối, theo dõi trạng thái và thực hiện các thao tác quản trị máy chấm công. Thiết bị được hiển thị theo tên, loại, firmware, địa chỉ mạng, serial, MAC, thời điểm hoạt động gần nhất và trạng thái Online/Offline.' -Size 10.5 | Out-Null

    Add-Heading -Text '2.1. Danh sách, tra cứu và trạng thái' -Level 2 | Out-Null
    Add-BulletList -Items @(
        'Tải danh sách thiết bị từ GET /api/v1/devices.',
        'Tìm kiếm tức thời theo tên thiết bị, địa chỉ IP, loại thiết bị hoặc số serial.',
        'Hiển thị thời điểm heartbeat/hoạt động cuối và badge Online/Offline.',
        'Nút Test trên mỗi dòng thực hiện kiểm tra kết nối và cập nhật lại danh sách sau khi có kết quả.'
    )
    Add-Heading -Text '2.2. Thêm và sửa cấu hình thiết bị' -Level 2 | Out-Null
    Add-Table -Rows @(
        @('Trường cấu hình', 'Mục đích / quy tắc'),
        @('Tên thiết bị', 'Tên hiển thị để người dùng phân biệt máy, ví dụ ZKTeco Phòng Chỉnh.'),
        @('Loại thiết bị', 'ZKTeco, Sunbeam hoặc Hikvision; adapter tương ứng sẽ được chọn ở server.'),
        @('IP và cổng', 'Địa chỉ TCP/IP; cổng mặc định của ZKTeco là 4370 nhưng có thể điều chỉnh.'),
        @('Serial SDK Pull', 'Serial dùng cho luồng kết nối trực tiếp/đọc dữ liệu.'),
        @('Serial ADMS Push', 'Serial mà máy dùng khi gọi/poll ADMS; có thể dùng fallback từ serial SDK ở luồng lưu hiện hành.'),
        @('Firmware, MAC, vị trí', 'Thông tin nhận diện, hiển thị và hỗ trợ vận hành.'),
        @('Bật ADMS Push', 'Chọn khi máy chủ động nhận/gửi lệnh qua ADMS thay vì chỉ dùng SDK/PULL.')
    ) | Out-Null
    Add-BulletList -Items @(
        'Thêm mới gọi POST /api/v1/devices; sửa gọi PUT /api/v1/devices/{id}.',
        'Cả hai thao tác đều ghi audit log và tải lại dữ liệu giao diện sau khi thành công.',
        'Xóa thiết bị gọi DELETE /api/v1/devices/{id}, có hộp xác nhận và yêu cầu quyền admin.'
    )

    Add-Heading -Text '2.3. Kiểm tra kết nối và khởi động lại' -Level 2 | Out-Null
    Add-Table -Rows @(
        @('Chức năng', 'Thao tác', 'Kết quả'),
        @('Test kết nối', 'Nhấn 🔌 Test trên dòng thiết bị hoặc gọi POST /api/v1/devices/{id}/test-connection.', 'Trả Online/Offline; nếu adapter hỗ trợ còn có số user, số log và firmware; kết quả được ghi audit.'),
        @('Xem trạng thái', 'GET /api/v1/devices/{id}/status.', 'Đọc trạng thái hiện tại mà không cần mở form cấu hình.'),
        @('Reboot', 'Nhấn 🔁 Reboot; POST /api/v1/devices/{id}/reboot.', 'Server gửi lệnh khởi động lại; thành công trả HTTP 202 và ghi audit. Thiết bị có thể Offline tạm thời trong lúc khởi động.'),
        @('Debug queue', 'GET /api/v1/devices/{id}/debug-queue.', 'Xem các lệnh đang chờ, hữu ích khi ADMS chưa lấy lệnh.')
    ) | Out-Null

    Add-Heading -Text '2.4. Sao chép dữ liệu giữa các thiết bị' -Level 2 | Out-Null
    Add-Paragraph -Text 'Thanh công cụ cho phép chọn một thiết bị nguồn và một hoặc nhiều thiết bị đích, sau đó nhấn ⚡ Sao chép. Mục tiêu là chuyển danh sách nhân viên cùng template vân tay sang máy đích, phù hợp khi triển khai nhiều máy có cùng danh sách.' -Size 10.5 | Out-Null
    Add-NumberList -Items @(
        'Chọn máy nguồn trong danh sách “Chọn nguồn sao chép”.',
        'Chọn một hoặc nhiều máy đích khác máy nguồn.',
        'Server lấy mapping nhân viên trên máy nguồn và lấy template từ database; nếu là SDK và database chưa đủ, server thử đọc các vị trí vân tay 0–9 từ máy nguồn.',
        'Tạo mapping trên máy đích, sau đó xếp lệnh ADMS hoặc kết nối SDK trực tiếp để đẩy USER và FINGERTEMPLATE.',
        'Giao diện thông báo thành công/thất bại và tải lại dữ liệu; không cho sao chép ngược vào chính máy nguồn.'
    )
    Add-Paragraph -Text 'API: POST /api/v1/devices/{sourceId}/backup với body { target_device_ids: [...] }. Đây là thao tác có thể ảnh hưởng nhiều thiết bị, nên cần kiểm tra máy đích và quyền truy cập trước khi chạy.' -Size 9.5 -SpaceAfter 8 | Out-Null

    Add-Heading -Text '2.5. Các năng lực thiết bị ở tầng API nền' -Level 2 | Out-Null
    Add-Paragraph -Text 'Trong giao diện bảng thiết bị hiện hành, một số nút Pull/Sync/Delete Log/Reset được ẩn theo yêu cầu UX; các endpoint vẫn tồn tại và được sử dụng từ mục Nhân viên hoặc công cụ quản trị.' -Size 10.5 | Out-Null
    Add-Table -Rows @(
        @('Năng lực', 'Endpoint', 'Ghi chú an toàn'),
        @('Kéo nhân viên', 'POST /api/v1/devices/{id}/pull-employees', 'Được trình bày trực tiếp ở mục Nhân viên; với SDK có thể đọc cả template vân tay.'),
        @('Đẩy/đồng bộ nhân viên', 'POST /api/v1/devices/{id}/sync-employees', 'Được trình bày ở thanh công cụ Nhân viên; ADMS thường xếp lệnh chờ máy polling.'),
        @('Xóa log chấm công', 'POST /api/v1/devices/{id}/clear-logs', 'Không thể hoàn tác; nên sao lưu/đối soát log trước.'),
        @('Reset toàn bộ máy', 'POST /api/v1/devices/{id}/reset', 'Xóa nhân viên, vân tay và log trên thiết bị; thao tác phá hủy, cần xác nhận mạnh.'),
        @('Hủy lệnh chờ', 'POST /api/v1/devices/{id}/cancel-pending-commands', 'Được dùng bởi nút Dừng quét; có thể hủy lệnh ADMS hoặc context SDK đang hoạt động.')
    ) | Out-Null

    Add-Heading -Text '2.6. Loại giao thức và yêu cầu kết nối' -Level 2 | Out-Null
    Add-BulletList -Items @(
        'ZKTeco: COM SDK/ZKemKeeper trên Windows cho các luồng SDK; kết nối TCP/IP thường qua cổng 4370.',
        'Sunbeam/Timmy và Hikvision: adapter HTTP/SDK tương ứng; thao tác reboot và đồng bộ phụ thuộc khả năng của từng model.',
        'ADMS: máy phải có serial đúng, bật ADMS và gọi về server để nhận hàng đợi; trạng thái Offline không nhất thiết xóa lệnh đã xếp hàng nhưng cần kiểm tra heartbeat.',
        'Không nên chạy đồng thời nhiều phiên COM trên cùng một IP:port; server có cơ chế điều phối phiên để tránh xung đột giữa quét, kéo và đồng bộ.'
    )

    # Nhân viên
    Add-PageBreak
    Add-Heading -Text '3. PHÂN HỆ NHÂN VIÊN' -Level 1 | Out-Null
    Add-Paragraph -Text 'Phân hệ Nhân viên quản lý hồ sơ nhân sự, trạng thái làm việc, quan hệ với thiết bị và toàn bộ vòng đời template vân tay. Các thao tác trên bảng có thể cập nhật ngay một người hoặc xử lý hàng loạt.' -Size 10.5 | Out-Null

    Add-Heading -Text '3.1. Danh sách, tìm kiếm và trạng thái' -Level 2 | Out-Null
    Add-BulletList -Items @(
        'Tải danh sách từ GET /api/v1/employees.',
        'Tìm theo họ tên, mã nhân viên, phòng ban hoặc chức danh.',
        'Hiển thị mã NV, avatar/tên, email, điện thoại, chức danh, phòng ban, ngày tham gia, trạng thái vân tay và trạng thái Active/Inactive.',
        'Badge vân tay hiển thị “Đã có” hoặc “Chưa có”; modal quản lý vân tay hiển thị số lượng mẫu thực tế.'
    )
    Add-Heading -Text '3.2. Thêm, sửa và xóa hồ sơ' -Level 2 | Out-Null
    Add-Table -Rows @(
        @('Nhóm dữ liệu', 'Trường / chức năng'),
        @('Định danh', 'Mã nhân viên (bắt buộc), họ tên đầy đủ (bắt buộc), mã số thẻ.'),
        @('Công việc', 'Chức danh, phòng ban, ngày nhận việc, trạng thái Active/Inactive.'),
        @('Liên hệ', 'Email, số điện thoại, giới tính, ngày sinh.'),
        @('Hiển thị', 'URL ảnh đại diện.'),
        @('Đăng ký ngay', 'Checkbox “Đồng bộ và yêu cầu quét vân tay trên máy ngay”, chọn thiết bị và PIN trên máy.'),
        @('API', 'POST /api/v1/employees; PUT /api/v1/employees/{id}; DELETE /api/v1/employees/{id}.')
    ) | Out-Null
    Add-Paragraph -Text 'Khi xóa, giao diện yêu cầu xác nhận và tải lại danh sách. Việc xóa hồ sơ web không nên được hiểu là đã xóa người dùng trên mọi máy; cần chạy luồng đồng bộ/xóa tương ứng nếu muốn làm sạch thiết bị.' -Size 9.5 | Out-Null

    Add-Heading -Text '3.3. Import danh sách từ Excel' -Level 2 | Out-Null
    Add-Paragraph -Text 'Nút 📊 Chọn Excel nhận file .xlsx, sau đó nút Import gửi multipart tới POST /api/v1/employees/import. Server đọc dòng tiêu đề và các cột mã NV, họ tên, mã thẻ, phòng ban, email, điện thoại, giới tính, ngày sinh, chức danh; kết quả trả số bản ghi thành công, thất bại và chi tiết lỗi theo dòng.' -Size 10.5 | Out-Null
    Add-BulletList -Items @(
        'Giới hạn request hiện hành: tối đa 10 MB.',
        'Mã nhân viên và họ tên là bắt buộc; dòng lỗi không làm mất các dòng đã import thành công.',
        'Sau khi import, giao diện tải lại danh sách và audit log ghi nhận kết quả bulk import.'
    )

    Add-Heading -Text '3.4. Chọn thiết bị cho các thao tác nhân viên' -Level 2 | Out-Null
    Add-Paragraph -Text 'Thanh công cụ có dropdown thiết bị, hiển thị tên, loại SDK/ADMS và trạng thái. Thiết bị được chọn là máy đích chính cho đăng ký một người, chọn để quét, quét hàng loạt, dừng quét và đồng bộ nhân viên. Cách chọn tập trung này giúp tránh gửi nhầm lệnh sang máy đã tắt hoặc sai model.' -Size 10.5 | Out-Null

    Add-Heading -Text '3.5. Đẩy nhân viên xuống máy' -Level 2 | Out-Null
    Add-Table -Rows @(
        @('Chức năng', 'Mô tả'),
        @('Đẩy tất cả', 'Chọn máy trong dropdown rồi nhấn 📤 Đẩy tất cả xuống máy; gọi POST /api/v1/devices/{id}/sync-employees cho toàn bộ nhân viên active.'),
        @('Đẩy một người', 'Menu ⋮ có thể mở luồng đồng bộ một nhân viên và cấp PIN/mapping cho thiết bị; API POST /api/v1/employees/{id}/devices/{deviceID}/sync.'),
        @('Đẩy một người ra mọi máy', 'Backend hỗ trợ POST /api/v1/employees/{id}/push-to-all-devices và trả số máy thành công/lỗi; helper vẫn có trong source dù nút không render ở mọi phiên bản giao diện.'),
        @('Đồng bộ vân tay', 'POST /api/v1/employees/{id}/fingerprints/push xếp các template đã lưu sang các máy trực tuyến/đích phù hợp.')
    ) | Out-Null
    Add-Paragraph -Text 'Với ADMS, “đã đẩy” thường có nghĩa là lệnh đã vào hàng đợi; máy chỉ thực sự cập nhật sau khi gọi getrequest và phản hồi xác nhận. Với SDK, server kết nối trực tiếp và ghi kết quả ngay khi adapter hoàn tất.' -Size 9.5 | Out-Null

    Add-Heading -Text '3.6. Kéo nhân viên và vân tay từ máy về web' -Level 2 | Out-Null
    Add-NumberList -Items @(
        'Nhấn 📥 Kéo NV từ máy ở toolbar hoặc nút cùng tên cạnh dropdown, rồi chọn thiết bị nguồn.',
        'Server kết nối và lấy user từ thiết bị; nhân viên mới được tạo, nhân viên cũ được ghép theo employee_code/PIN và cập nhật mapping.',
        'Đối với SDK/PULL, server nạp cache template một lần và đọc tất cả finger_index 0–9, không chỉ ngón 0.',
        'Mỗi template hợp lệ được upsert vào employee_fingerprint; sau đó cập nhật cờ fingerprint_enrolled và trạng thái mapping.',
        'Server đọc lại database để xác minh template đã lưu; giao diện trả số nhân viên mới, đã có và danh sách lỗi.'
    )
    Add-Paragraph -Text 'Đối với ADMS, server không đọc user trực tiếp theo kiểu COM; dữ liệu được hình thành từ mapping và các bản tin do máy gửi về. Nếu adapter đang bận hoặc circuit breaker mở, kết quả có thể chỉ có hồ sơ mà chưa có template; cần kiểm tra trạng thái kết nối và chạy lại sau khi phiên trước kết thúc.' -Size 9.5 | Out-Null

    Add-Heading -Text '3.7. Quét vân tay hàng loạt và chọn để quét' -Level 2 | Out-Null
    Add-Table -Rows @(
        @('Luồng', 'Thao tác trên giao diện', 'Xử lý server'),
        @('Quét VT hàng loạt', 'Chọn thiết bị, nhấn 🖐 Quét VT hàng loạt; hệ thống lọc nhân viên Active chưa có vân tay.', 'POST /api/v1/employees/batch-enroll với danh sách employee_ids và device_id; trả enqueued/total_requested/errors.'),
        @('Chọn để quét', 'Nhấn nút để bật checkbox trong bảng, chọn nhiều người, nhấn lại để bắt đầu.', 'Gửi đúng danh sách đã chọn qua cùng endpoint batch-enroll.'),
        @('Dừng quét', 'Chọn thiết bị và nhấn 🛑 Dừng quét.', 'POST /api/v1/devices/{id}/cancel-pending-commands; hủy lệnh ADMS chờ hoặc context SDK đang hoạt động.')
    ) | Out-Null
    Add-BulletList -Items @(
        'Máy xử lý từng người theo thứ tự; template chỉ được ghi khi máy xác nhận thành công.',
        'Người dùng có thể bỏ qua lượt không xác nhận và tiếp tục người tiếp theo theo cơ chế hàng đợi/timeout hiện hành.',
        'Nút dừng không xóa các template đã lưu trước đó; sau khi dừng nên tải lại danh sách và mở modal vân tay để kiểm tra.'
    )

    Add-Heading -Text '3.8. Quản lý vân tay từng nhân viên (tối đa 10 ngón)' -Level 2 | Out-Null
    Add-Table -Rows @(
        @('Thao tác', 'API / kết quả'),
        @('Xem mẫu', 'GET /api/v1/employees/{id}/fingerprints; hiển thị finger_index, kích thước template, thời điểm tạo và tổng số ngón.'),
        @('Đăng ký mới', 'POST /api/v1/employees/{id}/fingerprints/enroll với device_id; SDK/ZKemKeeper mở chế độ đăng ký trên máy và chờ template.'),
        @('Đăng ký lại', 'POST /api/v1/employees/{id}/fingerprints/re-enroll; dùng khi muốn thay mẫu hiện tại.'),
        @('Xóa một ngón', 'DELETE /api/v1/employees/{id}/fingerprints/{finger_index}; finger_index hợp lệ 0–9.'),
        @('Đẩy vân tay', 'POST /api/v1/employees/{id}/fingerprints/push; xếp template tới các thiết bị khác/đang online.'),
        @('Xóa dữ liệu enroll', 'POST /api/v1/employees/{id}/clear-enroll-data; gửi lệnh xóa enroll data trên các thiết bị ADMS.'),
        @('Xác nhận SDK cũ', 'GET device-mappings rồi POST /api/v1/employees/{id}/devices/{deviceID}/fingerprint-confirm để ghi nhận mapping/PIN đã có mẫu.')
    ) | Out-Null
    Add-Paragraph -Text 'Cơ sở dữ liệu hỗ trợ nhiều dòng cho cùng một nhân viên theo cặp (employee_id, finger_index), giới hạn index 0–9. Luồng kéo SDK hiện đọc đủ 10 vị trí và upsert từng vị trí; luồng đăng ký giao diện hiện hành đăng ký một lượt một ngón (thường bắt đầu ở index 0), vì vậy muốn đủ 10 ngón cần thực hiện các lượt đăng ký tương ứng hoặc dùng dữ liệu đã có trên máy để kéo về.' -Size 9.5 | Out-Null

    Add-Heading -Text '3.9. Đồng bộ một nhân viên và kích hoạt quét' -Level 2 | Out-Null
    Add-Paragraph -Text 'Modal “📥 Đồng bộ & Quét vân tay” cho phép chọn nhân viên, chọn máy nhận và nhập PIN trên máy. Server đồng bộ thông tin người dùng trước, tạo mapping, sau đó kích hoạt đăng ký. Giao diện chỉ chuyển sang trạng thái Đã có khi template được máy gửi về và lưu thành công; việc nút đã gửi lệnh không đồng nghĩa với việc đã có vân tay.' -Size 10.5 | Out-Null

    # Luồng tổng hợp
    Add-PageBreak
    Add-Heading -Text '4. LUỒNG ĐỒNG BỘ DỮ LIỆU VÀ VÂN TAY' -Level 1 | Out-Null
    Add-Heading -Text '4.1. Luồng cấu hình và kiểm tra thiết bị' -Level 2 | Out-Null
    Add-NumberList -Items @(
        'Tạo thiết bị với IP, port, serial và chế độ SDK/ADMS phù hợp.',
        'Nhấn Test để xác nhận server nhìn thấy máy và adapter đúng loại.',
        'Kiểm tra status/heartbeat; với ADMS xác nhận serial trên máy trùng cấu hình.',
        'Chỉ thực hiện pull, push hoặc enroll sau khi phiên kết nối trước đã kết thúc.'
    )
    Add-Heading -Text '4.2. Luồng Máy → Web (kéo nhân viên và template)' -Level 2 | Out-Null
    Add-Table -Rows @(
        @('Bước', 'Mô tả', 'Điểm kiểm tra'),
        @('1. Kết nối', 'SDK mở một phiên tới IP:port; ADMS nhận dữ liệu do máy gửi.', 'Không có lỗi not connected/circuit breaker.'),
        @('2. Đọc user', 'Lấy PIN, tên, quyền và thông tin cơ bản.', 'Số user trên máy khớp kỳ vọng.'),
        @('3. Đọc vân tay', 'SDK đọc cache một lần rồi quét index 0–9 cho từng PIN.', 'Log fingerprint_count và template_size > 0.'),
        @('4. Merge', 'Tạo/cập nhật employee và employee_device_mapping.', 'Không tạo trùng employee_code.'),
        @('5. Lưu/verify', 'Upsert employee_fingerprint, cập nhật cờ và đọc lại DB xác minh.', 'Modal web hiển thị Đã đăng ký (N ngón).'),
        @('6. Trả kết quả', 'API trả imported, existing, errors.', 'Nếu có lỗi, xử lý từng lỗi nhưng giữ các bản ghi thành công.')
    ) | Out-Null
    Add-Heading -Text '4.3. Luồng Web → Máy (đăng ký và đẩy)' -Level 2 | Out-Null
    Add-NumberList -Items @(
        'Chọn đúng thiết bị đích và kiểm tra Online/SDK/ADMS.',
        'Đồng bộ user/PIN trước nếu người đó chưa tồn tại trên máy.',
        'Gửi lệnh enroll; người dùng đặt ngón tay theo hướng dẫn của máy.',
        'Chờ máy xác nhận template; server lưu database và cập nhật badge.',
        'Nếu cần, đẩy template đã lưu sang các máy khác hoặc dùng backup nguồn–đích.'
    )
    Add-Paragraph -Text 'Trong ADMS, server có thể trả “đã đưa vào queue” trước khi thiết bị thực sự nhận lệnh. Cần chờ máy gọi getrequest và kiểm tra phản hồi/biometric status. Trong SDK, lệnh có thể thất bại ngay nếu máy tắt, COM chưa kết nối hoặc một phiên enroll khác còn active.' -Size 9.5 | Out-Null

    # Quyền và vận hành
    Add-PageBreak
    Add-Heading -Text '5. PHÂN QUYỀN, NHẬT KÝ VÀ YÊU CẦU VẬN HÀNH' -Level 1 | Out-Null
    Add-Heading -Text '5.1. Phân quyền' -Level 2 | Out-Null
    Add-BulletList -Items @(
        'Các thao tác tạo/sửa/xóa thiết bị và nhân viên, import Excel, test, reboot, pull, sync, batch enroll và các thao tác phá hủy nằm trong nhóm RequireRole("admin") ở route tương ứng.',
        'Các endpoint danh sách/trạng thái cơ bản có thể được gọi để hiển thị giao diện; quyền thực tế vẫn phụ thuộc middleware xác thực của ứng dụng.',
        'Không chia sẻ tài khoản admin cho thao tác hàng ngày; nên phân tách người cấu hình máy và người vận hành quét nếu hệ thống triển khai nhiều vai trò.'
    )
    Add-Heading -Text '5.2. Audit log và thông báo' -Level 2 | Out-Null
    Add-BulletList -Items @(
        'Các sự kiện quan trọng như tạo/sửa/xóa thiết bị, test, reboot, pull nhân viên, batch enroll, xóa enroll data được ghi audit log.',
        'Giao diện dùng toast/message để báo đang xử lý, thành công, cảnh báo và lỗi; nút được vô hiệu hóa trong lúc request đang chạy.',
        'Khi kiểm tra lỗi vân tay, cần phân biệt ba trạng thái: đã xếp lệnh, máy đã nhận lệnh và template đã lưu database.'
    )
    Add-Heading -Text '5.3. Điều kiện vận hành bắt buộc' -Level 2 | Out-Null
    Add-Table -Rows @(
        @('Điều kiện', 'Mô tả kiểm tra'),
        @('Windows và COM SDK', 'Luồng ZKTeco SDK cần Windows, DLL ZKemKeeper đã đăng ký và tiến trình có thể khởi tạo COM.'),
        @('Mạng thiết bị', 'Server phải tới được IP:port; firewall không chặn cổng thiết bị (thường 4370).'),
        @('ADMS', 'Serial ADMS, URL server và heartbeat phải đúng; máy phải polling để nhận lệnh trong hàng đợi.'),
        @('Mapping/PIN', 'PIN trên web phải trùng user ID trên máy; nếu nhiều mapping cần chọn đúng thiết bị.'),
        @('Không chạy chồng phiên', 'Không chạy kéo, enroll, reboot hoặc backup đồng thời trên cùng máy SDK; chờ phiên trước hoàn tất.'),
        @('Backup trước thao tác phá hủy', 'Clear log và reset không thể hoàn tác; nên sao chép/đối soát trước.'),
        @('Đủ 10 ngón', 'Pull SDK có thể đọc index 0–9; enroll giao diện là từng lượt một ngón và cần xác nhận sau mỗi lượt.')
    ) | Out-Null

    # Checklist
    Add-PageBreak
    Add-Heading -Text '6. CHECKLIST KIỂM TRA NGHIỆM THU' -Level 1 | Out-Null
    Add-Paragraph -Text 'Có thể dùng checklist sau khi bàn giao hoặc sau khi thay đổi cấu hình máy. Đánh dấu Đạt/Không đạt và ghi request_id hoặc ảnh chụp log khi cần truy vết.' -Size 10.5 | Out-Null
    Add-Table -Rows @(
        @('STT', 'Hạng mục', 'Tiêu chí đạt'),
        @('1', 'Khai báo thiết bị', 'Tên, loại, IP, port, serial và chế độ ADMS đúng; lưu không lỗi.'),
        @('2', 'Test kết nối', 'Trả Online; không có lỗi COM/timeout; thông tin firmware/user/log hợp lý.'),
        @('3', 'Tạo nhân viên', 'Hồ sơ xuất hiện trong danh sách với mã NV duy nhất.'),
        @('4', 'Import Excel', 'Số dòng thành công/thất bại đúng; lỗi theo dòng rõ ràng.'),
        @('5', 'Đẩy nhân viên', 'Thiết bị nhận USER; mapping/PIN đúng trên máy.'),
        @('6', 'Đăng ký một ngón', 'Máy hiện màn hình quét đủ thời gian; sau xác nhận modal hiển thị Đã đăng ký (1 ngón).'),
        @('7', 'Kéo đủ template', 'Log/API cho thấy fingerprint_count; database có các index 0–9 thực tế có mẫu.'),
        @('8', 'Quét hàng loạt', 'Danh sách xử lý lần lượt; người bỏ lượt không chặn người tiếp theo; dừng quét hủy phần còn chờ.'),
        @('9', 'Đồng bộ vân tay', 'Máy đích nhận template; kiểm tra sinh trắc học trực tiếp trên máy.'),
        @('10', 'Backup/reset', 'Backup đúng nguồn–đích; reset chỉ chạy sau khi có xác nhận và đã lưu dữ liệu cần thiết.'),
        @('11', 'Audit', 'Các thao tác quản trị có bản ghi người thực hiện, đối tượng và kết quả.' )
    ) | Out-Null

    Add-Heading -Text '7. KẾT LUẬN VÀ ĐỀ XUẤT SỬ DỤNG' -Level 1 | Out-Null
    Add-Paragraph -Text 'Hai phân hệ Thiết bị và Nhân viên đã bao phủ đầy đủ vòng đời vận hành: khai báo máy, kiểm tra kết nối, đồng bộ hồ sơ, đăng ký/quản lý vân tay, kéo dữ liệu về web, đẩy sang máy khác và theo dõi kết quả. Điểm quan trọng nhất khi vận hành là chọn đúng giao thức và thiết bị, chờ xác nhận thực tế từ máy trước khi kết luận template đã lưu, đồng thời không chạy chồng các phiên COM trên cùng máy.' -Size 10.5 | Out-Null
    Add-Paragraph -Text 'Đề xuất: dùng Test trước mọi luồng; sau Pull mở modal vân tay để kiểm tra số ngón; sau Push/ADMS chờ máy polling và kiểm tra trên thiết bị; trước Clear log/Reset luôn backup và lưu audit/request_id.' -Size 10.5 | Out-Null
    Add-Paragraph -Text 'Báo cáo này là tài liệu mô tả chức năng theo mã nguồn và giao diện tại thời điểm 17/07/2026. Khi thay đổi model máy, phiên bản SDK hoặc giao diện, nên cập nhật lại các bảng endpoint và checklist tương ứng.' -Size 9.5 -SpaceAfter 10 | Out-Null

    Add-FieldFooter
    $doc.SaveAs2($outPath, 16) # wdFormatDocumentDefault (.docx)
    Write-Output "Created: $outPath"
}
finally {
    if ($doc -ne $null) {
        try { $doc.Close($false) } catch { }
        try { [Runtime.InteropServices.Marshal]::ReleaseComObject($doc) | Out-Null } catch { }
    }
    if ($word -ne $null) {
        try { $word.Quit() } catch { }
        try { [Runtime.InteropServices.Marshal]::ReleaseComObject($word) | Out-Null } catch { }
    }
    [GC]::Collect()
    [GC]::WaitForPendingFinalizers()
}
