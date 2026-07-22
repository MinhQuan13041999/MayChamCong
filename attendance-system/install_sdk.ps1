# ==========================================
# AUTO INSTALL & REGISTER ZKTeco COM SDK
# ==========================================
# Cai dat tu dong ZKTeco SDK tu du an
# Chay script nay voi quyen Admin
# ==========================================

param(
    [ValidateSet("x64", "x86")]
    [string]$Architecture = "x64"  # "x64" cho 64-bit, "x86" cho 32-bit
)

$ErrorActionPreference = "Stop"

# ==========================================
# 1. KIỂM TRA QUYỀN ADMIN
# ==========================================
Write-Host "=== Kiem Tra Quyen Admin ===" -ForegroundColor Cyan
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")

if (-not $isAdmin) {
    Write-Host "❌ Loi: Script phai chay voi quyen Admin!" -ForegroundColor Red
    Write-Host "Cach sua:" -ForegroundColor Yellow
    Write-Host "  1. Mo PowerShell voi quyen Admin (Win+X -> Windows PowerShell Admin)"
    Write-Host "  2. Chay lai script"
    Exit 1
}

Write-Host "✅ Co quyen Admin" -ForegroundColor Green

# ==========================================
# 2. KIỂM TRA HỆ THỐNG
# ==========================================
Write-Host "`n=== Kiem Tra He Thong ===" -ForegroundColor Cyan

if ([System.Environment]::Is64BitOperatingSystem) {
    $osArchitecture = "x64"
} else {
    $osArchitecture = "x86"
}
Write-Host "He dieu hanh: $osArchitecture" -ForegroundColor Yellow

if ($Architecture -eq "x86" -and $osArchitecture -eq "x64") {
    Write-Host "⚠️  Canh bao: OS la 64-bit nhung ban chon cai 32-bit SDK" -ForegroundColor Yellow
    Write-Host "Khuyen nghi: Cai 64-bit (x64) thay vi 32-bit" -ForegroundColor Yellow
}

# ==========================================
# 3. ĐỊNH NGHĨA ĐƯỜNG DẪN
# ==========================================
Write-Host "`n=== Thiet Lap Duong Dan ===" -ForegroundColor Cyan

# Tim project root chua thu muc ZKTecoStandAlonePullSDK
$projectRoot = $PSScriptRoot
if (-not (Test-Path (Join-Path $projectRoot "ZKTecoStandAlonePullSDK"))) {
    $projectRoot = Split-Path -Parent $PSScriptRoot
}

$archFolder = if ($Architecture -eq "x64") { "64bit" } else { "32bit" }
$sdkSource = Join-Path $projectRoot "ZKTecoStandAlonePullSDK" "SDK-Ver6.3.1.37" $archFolder
$sdkDest = "C:\Program Files\ZKTeco\ZKemkeeper"

Write-Host "Thu muc nguon SDK: $sdkSource" -ForegroundColor Yellow
Write-Host "Thu muc dich: $sdkDest" -ForegroundColor Yellow

# ==========================================
# 4. KIỂM TRA FILE SDK TỒN TẠI
# ==========================================
Write-Host "`n=== Kiem Tra File SDK ===" -ForegroundColor Cyan

if (-not (Test-Path $sdkSource)) {
    Write-Host "❌ Loi: Khong tim thay thu muc SDK" -ForegroundColor Red
    Write-Host "Duong dan: $sdkSource" -ForegroundColor Red
    Exit 1
}

$dllFile = Join-Path $sdkSource "zkemkeeper.dll"
if (-not (Test-Path $dllFile)) {
    Write-Host "❌ Loi: Khong tim thay file zkemkeeper.dll" -ForegroundColor Red
    Exit 1
}

Write-Host "✅ Tim thay SDK tai: $sdkSource" -ForegroundColor Green
Write-Host "✅ Tim thay DLL: $dllFile" -ForegroundColor Green

# ==========================================
# 5. TẠO THỰC MỤC ĐÍCH
# ==========================================
Write-Host "`n=== Tao Thu Muc Dich ===" -ForegroundColor Cyan

if (-not (Test-Path $sdkDest)) {
    Write-Host "Tao thu muc: $sdkDest" -ForegroundColor Yellow
    New-Item -ItemType Directory -Path $sdkDest -Force | Out-Null
    Write-Host "✅ Thu muc tao thanh cong" -ForegroundColor Green
} else {
    Write-Host "✅ Thu muc da ton tai" -ForegroundColor Green
}

# ==========================================
# 6. SAO CHÉP CÁC FILE DLL
# ==========================================
Write-Host "`n=== Sao Chep File SDK ===" -ForegroundColor Cyan

$dllFiles = @(
    "zkemkeeper.dll",
    "zkemsdk.dll",
    "tcpcomm.dll",
    "commpro.dll",
    "rscagent.dll",
    "rscomm.dll",
    "plcommpro.dll",
    "plrscomm.dll",
    "pltcpcomm.dll"
)

foreach ($dll in $dllFiles) {
    $sourceFile = Join-Path $sdkSource $dll
    $destFile = Join-Path $sdkDest $dll
    
    if (Test-Path $sourceFile) {
        Write-Host "Sao chep: $dll" -ForegroundColor Yellow
        Copy-Item -Path $sourceFile -Destination $destFile -Force | Out-Null
        Write-Host "  ✅ OK" -ForegroundColor Green
    }
}

Write-Host "✅ Sao chep file thanh cong" -ForegroundColor Green

# ==========================================
# 7. ĐĂNG KÝ DLL
# ==========================================
Write-Host "`n=== Dang Ky DLL ===" -ForegroundColor Cyan

$zkemkeeperDll = Join-Path $sdkDest "zkemkeeper.dll"
Write-Host "Dang ky: $zkemkeeperDll" -ForegroundColor Yellow

try {
    # Phuong phap 1: Dung regsvr32.exe
    $regResult = & regsvr32.exe /s $zkemkeeperDll 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Dang ky DLL thanh cong" -ForegroundColor Green
    } else {
        Write-Host "⚠️  Canh bao: regsvr32 tra ve ma loi $LASTEXITCODE" -ForegroundColor Yellow
    }
} catch {
    Write-Host "❌ Loi khi dang ky DLL: $_" -ForegroundColor Red
    Exit 1
}

# ==========================================
# 8. TEST KẾT NỐI COM SDK
# ==========================================
Write-Host "`n=== Test Ket Noi COM SDK ===" -ForegroundColor Cyan

try {
    Write-Host "Tao doi tuong COM zkemkeeper.ZKEM.1..." -ForegroundColor Yellow
    $zkem = New-Object -ComObject zkemkeeper.ZKEM.1
    Write-Host "✅ COM SDK da duoc dang ky thanh cong!" -ForegroundColor Green
    
    # Thu lay thong tin
    $ver = ""
    if ($zkem.GetSDKVersion([ref]$ver)) {
        Write-Host "Phien ban SDK: $ver" -ForegroundColor Green
    }
} catch {
    Write-Host "❌ Loi khi test COM SDK: $_" -ForegroundColor Red
    Write-Host "Nguyen nhan co the:" -ForegroundColor Yellow
    Write-Host "  - DLL chua dang ky dung cach" -ForegroundColor Yellow
    Write-Host "  - Can khoi dong lai may tinh" -ForegroundColor Yellow
    Write-Host "  - Quyen truy cap han che" -ForegroundColor Yellow
}

# ==========================================
# 9. KIỂM TRA REGISTRY
# ==========================================
Write-Host "`n=== Kiem Tra Registry ===" -ForegroundColor Cyan

try {
    $regPath = "HKLM:\Software\Classes\zkemkeeper.ZKEM.1"
    if (Test-Path $regPath) {
        Write-Host "✅ Entry Registry tim thay" -ForegroundColor Green
        Get-Item $regPath -ErrorAction SilentlyContinue | Format-List
    } else {
        Write-Host "⚠️  Entry Registry khong tim thay" -ForegroundColor Yellow
    }
} catch {
    Write-Host "⚠️  Khong the kiem tra registry" -ForegroundColor Yellow
}

# ==========================================
# 10. HOÀN THÀNH
# ==========================================
Write-Host "`n" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "✅ HOAN TAT CAI DAT SDK" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan

Write-Host "`nThong tin cai dat:" -ForegroundColor Cyan
Write-Host "  - Kien truc: $Architecture" -ForegroundColor Yellow
Write-Host "  - SDK: $sdkDest" -ForegroundColor Yellow
Write-Host "  - DLL: zkemkeeper.dll" -ForegroundColor Yellow

Write-Host "`nBuoc tiep theo:" -ForegroundColor Cyan
Write-Host "  1. ✅ Mo http://localhost:8085/" -ForegroundColor Green
Write-Host "  2. ✅ Dang nhap (admin/admin123)" -ForegroundColor Green
Write-Host "  3. ✅ Vao tab '🖥 Thiet Bi' -> '➕ Them thiet bi'" -ForegroundColor Green
Write-Host "  4. ✅ Dien thong tin thiet bi ZKTeco" -ForegroundColor Green
Write-Host "  5. ✅ Bo tich 'Bat ADMS Push'" -ForegroundColor Green
Write-Host "  6. ✅ Luu & test ket noi" -ForegroundColor Green

Write-Host "`n⚠️  Luu y:" -ForegroundColor Yellow
Write-Host "  - Neu loi COM SDK, thu khoi dong lai PowerShell" -ForegroundColor Yellow
# Note: we removed the Read-Host at the end so it executes synchronously without blocking standard input in scripts.
