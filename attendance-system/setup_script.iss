; Script Inno Setup đóng gói Hệ thống Chấm công Doanh nghiệp thành file cài đặt tự động .EXE
; Tải Inno Setup Compiler tại: https://jrsoftware.org/isdl.php

[Setup]
AppName=Hệ Thống Chấm Công Doanh Nghiệp
AppVersion=1.0
AppPublisher=Phát Triển Phần Mềm
DefaultDirName={pf}\AttendanceSystem
DefaultGroupName=Hệ Thống Chấm Công
UninstallDisplayIcon={app}\server.exe
Compression=lzma2
SolidCompression=yes
OutputDir=.\Output
OutputBaseFilename=AttendanceSystem_Setup
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64

[Files]
; Sao chép các tệp tin thực thi và cấu hình
Source: "dist_package\server.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist_package\config.yaml"; DestDir: "{app}"; Flags: ignoreversion
Source: "dist_package\nssm.exe"; DestDir: "{app}"; Flags: ignoreversion

; Sao chép thư mục Web UI và Database Migrations
Source: "dist_package\web\*"; DestDir: "{app}\web"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "dist_package\migrations\*"; DestDir: "{app}\migrations"; Flags: ignoreversion recursesubdirs createallsubdirs

; Đóng gói các DLL SDK ZKTeco kết nối thiết bị và tự động đăng ký với Windows Registry (cờ regserver)
Source: "dist_package\sdk\zkemkeeper.dll"; DestDir: "{sys}"; Flags: restartreplace sharedfile regserver
Source: "dist_package\sdk\commpro.dll"; DestDir: "{sys}"; Flags: restartreplace sharedfile
Source: "dist_package\sdk\comms.dll"; DestDir: "{sys}"; Flags: restartreplace sharedfile
Source: "dist_package\sdk\rscomm.dll"; DestDir: "{sys}"; Flags: restartreplace sharedfile
Source: "dist_package\sdk\tcpcomm.dll"; DestDir: "{sys}"; Flags: restartreplace sharedfile
Source: "dist_package\sdk\usbcomm.dll"; DestDir: "{sys}"; Flags: restartreplace sharedfile

[Icons]
; Tạo Shortcut ngoài Desktop và Start Menu
Name: "{group}\Hệ Thống Chấm Công"; Filename: "{app}\server.exe"
Name: "{group}\Gỡ cài đặt phần mềm"; Filename: "{uninstallexe}"
Name: "{commondesktop}\Hệ Thống Chấm Công"; Filename: "{app}\server.exe"

[Run]
; Đăng ký ứng dụng chạy ngầm dưới dạng Windows Service bằng nssm
Filename: "{app}\nssm.exe"; Parameters: "install AttendanceService ""{app}\server.exe"""; Flags: runhidden
Filename: "{app}\nssm.exe"; Parameters: "set AttendanceService AppDirectory ""{app}"""; Flags: runhidden
Filename: "{app}\nssm.exe"; Parameters: "set AttendanceService Start SERVICE_AUTO_START"; Flags: runhidden
Filename: "{sys}\net.exe"; Parameters: "start AttendanceService"; Flags: runhidden

[UninstallRun]
; Dừng service và gỡ cài đặt hoàn toàn khỏi hệ thống khi xóa app
Filename: "{sys}\net.exe"; Parameters: "stop AttendanceService"; Flags: runhidden
Filename: "{app}\nssm.exe"; Parameters: "remove AttendanceService confirm"; Flags: runhidden
