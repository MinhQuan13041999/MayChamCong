
// Theme Switcher Functions
function initTheme() {
  const savedTheme = localStorage.getItem('app_theme') || 'dark';
  if (savedTheme === 'light') {
    document.body.classList.add('light-theme');
  } else {
    document.body.classList.remove('light-theme');
  }
  updateThemeBtnText();
}

function toggleTheme() {
  const isLight = document.body.classList.toggle('light-theme');
  localStorage.setItem('app_theme', isLight ? 'light' : 'dark');
  updateThemeBtnText();
  if (typeof updateCharts === 'function') {
    updateCharts();
  }
}

function updateThemeBtnText() {
  const btn = document.getElementById('themeToggleBtn');
  if (!btn) return;
  const isLight = document.body.classList.contains('light-theme');
  if (window._lang === 'vi') {
    btn.innerHTML = isLight ? '🌙 Tối' : '☀️ Sáng (Enterprise)';
    btn.title = isLight ? 'Chuyển sang giao diện Tối' : 'Chuyển sang giao diện Sáng Doanh nghiệp';
  } else {
    btn.innerHTML = isLight ? '🌙 Dark' : '☀️ Light (Enterprise)';
    btn.title = isLight ? 'Switch to Dark Theme' : 'Switch to Enterprise Light Theme';
  }
}

window.toggleTheme = toggleTheme;
window.initTheme = initTheme;
window.updateThemeBtnText = updateThemeBtnText;

const fallbackDevices = [
  { id: 'dev-1', name: 'ZKTeco tầng 1', device_type: 'zkteco', ip_address: '192.168.1.100', port: 4370, serial_number: 'ZK-001', serial_number_adms: 'ZK-001', adms_enabled: true, location: 'Tầng 1', status: 'online' },
  { id: 'dev-2', name: 'Sunbeam văn phòng', device_type: 'sunbeam', ip_address: '192.168.1.150', port: 80, serial_number: 'SB-002', serial_number_adms: 'SB-002', adms_enabled: true, location: 'Văn phòng', status: 'offline' },
  { id: 'dev-3', name: 'Hikvision cổng chính', device_type: 'hikvision', ip_address: '192.168.1.200', port: 80, serial_number: 'HK-003', serial_number_adms: 'HK-003', adms_enabled: true, location: 'Cổng chính', status: 'online' }
];

const fallbackEmployees = [
  { id: 'emp-1', employee_code: 'NV001', full_name: 'Nguyễn Văn An', department_id: 'IT', card_no: '1001', status: 'active' },
  { id: 'emp-2', employee_code: 'NV002', full_name: 'Trần Thị Bình', department_id: 'HR', card_no: '1002', status: 'active' }
];

const fallbackAttendance = [
  { id: 1, employee_code: 'NV001', full_name: 'Nguyễn Văn An', check_time: '2026-07-13T08:00:00Z', check_type: 'in', device_id: 'dev-1' },
  { id: 2, employee_code: 'NV002', full_name: 'Trần Thị Bình', check_time: '2026-07-13T17:30:00Z', check_type: 'out', device_id: 'dev-3' }
];

const fallbackHistory = [
  { id: 1, device_id: 'dev-1', sync_type: 'attendance', trigger_type: 'manual', status: 'success', record_count: 24 },
  { id: 2, device_id: 'dev-3', sync_type: 'employee', trigger_type: 'manual', status: 'success', record_count: 2 }
];

const state = {
  token: localStorage.getItem('attendance-token') || '',
  user: JSON.parse(localStorage.getItem('attendance-user') || 'null'),
  activeView: 'dashboard',
  devices: [],
  employees: [],
  attendance: [],
  attendanceSummary: [],
  syncHistory: [],
  dashboardStats: {},
  editingDeviceId: '',
  editingEmployeeId: '',
  editingShiftId: '',
  editingRotationId: '',
  selectedFingerprintEmployeeId: null,
  selectedFingerprintFingerIndex: null,
  fingerprintTemplates: [],
  lastScan: null,
  shifts: [],
  essRequests: [],
  rotationPatterns: [],
  employeeShifts: [],
  shiftSwaps: [],
  selectedShiftFilter: 'all',
  registeredFaces: [],
  faceMatcher: null,
  faceModelsLoaded: false,
  cameraStream: null,
  registerCameraStream: null,
  faceDetectTimer: null,
  registerFaceDetectTimer: null,
  lastRecognizedTime: {}
};

// Inline batch-select mode for employee list
state.batchSelectMode = false;
state.batchSelected = {}; // map employeeID -> true

function getLocalDateStr(dateOrStr) {
  if (!dateOrStr) return '';
  if (typeof dateOrStr === 'string') {
    if (dateOrStr.length <= 10) return dateOrStr.substring(0, 10);
    const date = new Date(dateOrStr);
    const y = date.getFullYear();
    const m = String(date.getMonth() + 1).padStart(2, '0');
    const d = String(date.getDate()).padStart(2, '0');
    return `${y}-${m}-${d}`;
  }
  const y = dateOrStr.getFullYear();
  const m = String(dateOrStr.getMonth() + 1).padStart(2, '0');
  const d = String(dateOrStr.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

function findEmployeeByCode(employeeCode) {
  if (!employeeCode) return null;
  return state.employees.find((employee) => employee.employee_code === employeeCode) || null;
}

function getEmployeeInitials(name) {
  return (name || '?')
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .slice(-2)
    .map((part) => part.charAt(0).toUpperCase())
    .join('') || '?';
}

// Cập nhật thẻ thông tin chung trên giao diện khi máy gửi một lượt nhận diện mới.
function showLastScan(scan) { renderFullLastScanCard(scan); }

const els = {};
let eventSource = null;
let attendanceRefreshTimer = null;
let attendanceRefreshInFlight = false;

function startAttendanceAutoRefresh() {
  if (attendanceRefreshTimer) return;
  attendanceRefreshTimer = window.setInterval(async () => {
    if (!state.token || state.activeView !== 'attendance' || attendanceRefreshInFlight) return;
    attendanceRefreshInFlight = true;
    try {
      await loadAttendance();
    } finally {
      attendanceRefreshInFlight = false;
    }
  }, 10000);
}

function connectSSE() {
  if (eventSource) {
    try { eventSource.close(); } catch(e) {}
  }
  if (!state.token) return;

  const sseUrl = `/api/v1/stream?token=${encodeURIComponent(state.token)}`;
  eventSource = new EventSource(sseUrl);

  eventSource.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data);
      if (payload.type === 'ping') return;
      console.log('SSE Real-time Event received:', payload);
      if (payload.type === 'attendance_synced') {
        const d = payload.data || {};
        if (!d.inserted || d.inserted <= 0) {
          // Bỏ qua nếu không có bản ghi mới để tránh spam thông báo dữ liệu đã tồn tại
          return;
        }
        showLastScan(d);
        // Nhân viên có thể vừa được thêm trên máy; tải lại danh sách để bổ sung
        // ảnh, phòng ban và liên hệ nếu dữ liệu phía trình duyệt chưa có.
        if (d.latest_employee_code && !findEmployeeByCode(d.latest_employee_code)) {
          loadEmployees().then(() => showLastScan(d)).catch(() => {});
        }
        const deviceName = d.device_name || 'máy chấm công';
        const empCode = d.latest_employee_code ? ` (NV: ${d.latest_employee_code})` : '';
        let checkTimeStr = '';
        if (d.latest_check_time) {
          try { checkTimeStr = ` lúc ${new Date(d.latest_check_time).toLocaleTimeString('vi-VN')}`; } catch(e){}
        }
        const msg = `\u26a1 Quét thành công${empCode}${checkTimeStr} từ "${deviceName}" — Đã lưu ${d.inserted} log mới!`;
        showToast(msg, 'success');
        // Luôn tải lại để bảng chấm công cập nhật ngay
        loadAttendance().then(() => renderAttendance()).catch(()=>{});
      } else if (payload.type === 'attendance_processed') {
        const msg = window._lang === 'vi'
          ? `\u26a1 H\u1ec7 th\u1ed1ng v\u1eeba ho\u00e0n t\u1ea5t t\u00ednh to\u00e1n c\u00f4ng cho ng\u00e0y ${payload.data.date}!`
          : `\u26a1 System just finished calculating attendance for date ${payload.data.date}!`;
        showToast(msg, 'success');
        loadAll();
      } else if (payload.type === 'monthly_report_progress') {
        const progress = payload.data || {};
        const label = progress.employee_code || '';
        const status = progress.status || '';
        if (els.reportMessage && state.activeView === 'reports') {
          els.reportMessage.className = 'message info';
          els.reportMessage.textContent = `${window._lang === 'vi' ? 'Đang gửi báo cáo' : 'Sending report'} ${label} (${status})...`;
        }
      } else if (payload.type === 'fingerprint_synced') {
        const msg = window._lang === 'vi'
          ? `\ud83d\udc4b ADMS: V\u00e2n tay \u0111\u00e3 \u0111\u01b0\u1ee3c \u0111\u1eb7t l\u1ecbch ${payload.data.commands} l\u1ec7nh \u0111\u1ed3ng b\u1ed9 t\u1edbi c\u00e1c m\u00e1y kh\u00e1c!`
          : `\ud83d\udc4b ADMS: Fingerprint queued ${payload.data.commands} sync commands to other devices!`;
        showToast(msg, 'info');
      } else if (payload.type === 'fingerprint_updated') {
        const msg = window._lang === 'vi'
          ? `\u2705 \u0110\u00e3 nh\u1eadn m\u1eabu v\u00e2n tay m\u1edbi cho nh\u00e2n vi\u00ean!`
          : `\u2705 Received new fingerprint template for employee!`;
        showToast(msg, 'success');
        loadAll();
        if (state.selectedFingerprintEmployeeId === payload.data.employee_id) {
          openFingerprintModal(payload.data.employee_id);
        }
      } else if (payload.type === 'fingerprint_enroll_skipped') {
        const d = payload.data || {};
        const detail = d.error ? ` (${d.error})` : '';
        const msg = window._lang === 'vi'
          ? `\u26a0 Kh\u00f4ng nh\u1eadn \u0111\u01b0\u1ee3c v\u00e2n tay trong 10 gi\u00e2y; ${d.batch ? '\u0111ang chuy\u1ec3n sang nh\u00e2n vi\u00ean ti\u1ebfp theo' : 'l\u01b0\u1ee3t \u0111\u0103ng k\u00fd \u0111\u00e3 k\u1ebft th\u00fac'}.${detail}`
          : `\u26a0 No fingerprint was received within 10 seconds; ${d.batch ? 'moving to the next employee' : 'enrollment ended'}.${detail}`;
        showToast(msg, 'warning');
      } else if (payload.type === 'fingerprint_enroll_stopped') {
        const msg = window._lang === 'vi'
          ? `\u23f9 \u0110\u00e3 d\u1eebng \u0111\u0103ng k\u00fd v\u00e2n tay tr\u00ean m\u00e1y.`
          : `\u23f9 Fingerprint enrollment was stopped on the device.`;
        showToast(msg, 'info');
      } else if (payload.type === 'employees_deleted') {
        loadEmployees().then(renderEmployees).catch(() => {});
      } else if (payload.type === 'face_attendance_detected') {
        showFaceAttendanceToast(payload.data);
        appendFaceAttendanceLogRow(payload.data);
      } else if (payload.type === 'face_updated') {
        loadEmployees().then(renderEmployees).catch(() => {});
        loadRegisteredFaces().catch(() => {});
      } else if (payload.type === 'batch_enroll_queued') {
        const d = payload.data || {};
        const msg = window._lang === 'vi'
          ? `🖐 Đã đưa ${d.enqueued}/${d.total_requested} lệnh quét vân tay vào hàng đợi máy "${d.device_name}"!`
          : `🖐 Queued ${d.enqueued}/${d.total_requested} enroll commands to "${d.device_name}"!`;
        showToast(msg, 'success');
      } else if (payload.type === 'fingerprint_scanned_realtime') {
        const d = payload.data || {};
        console.log('⚡ Fingerprint Realtime Scan Event:', d);
        playSuccessBeep();
        showRealtimeScanModal(d);
        showLastScan({
          latest_employee_code: d.employee_code,
          latest_check_time: d.check_time,
          latest_check_type: d.check_type,
          latest_verify_mode: d.verify_mode,
          device_name: d.device_name
        });
        loadAttendance().then(() => renderAttendance()).catch(()=>{});
      }
    } catch (err) {
      console.error('Error parsing SSE event:', err);
    }
  };

  eventSource.onerror = (err) => {
    console.warn('SSE connection lost. Retrying in 5 seconds...', err);
    if (eventSource) {
      try { eventSource.close(); } catch(e) {}
    }
    setTimeout(connectSSE, 5000);
  };
}

function playSuccessBeep() {
  try {
    const AudioContext = window.AudioContext || window.webkitAudioContext;
    if (!AudioContext) return;
    const ctx = new AudioContext();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    
    osc.type = 'sine';
    osc.frequency.setValueAtTime(880, ctx.currentTime);
    osc.frequency.exponentialRampToValueAtTime(1320, ctx.currentTime + 0.12);
    
    gain.gain.setValueAtTime(0.25, ctx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.25);
    
    osc.connect(gain);
    gain.connect(ctx.destination);
    
    osc.start(ctx.currentTime);
    osc.stop(ctx.currentTime + 0.25);
  } catch(e) {
    console.log('Audio notification error:', e);
  }
}

function showRealtimeScanModal(data) {
  let modal = document.getElementById('realtimeScanToast');
  if (!modal) {
    modal = document.createElement('div');
    modal.id = 'realtimeScanToast';
    modal.style.cssText = `
      position: fixed;
      top: 24px;
      left: 50%;
      transform: translateX(-50%) translateY(-20px);
      z-index: 99999;
      background: rgba(15, 23, 42, 0.95);
      border: 1px solid rgba(16, 185, 129, 0.5);
      box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4), 0 0 20px rgba(16, 185, 129, 0.3);
      backdrop-filter: blur(16px);
      color: #fff;
      padding: 16px 24px;
      border-radius: 16px;
      display: flex;
      align-items: center;
      gap: 16px;
      min-width: 360px;
      max-width: 480px;
      opacity: 0;
      transition: all 0.35s cubic-bezier(0.16, 1, 0.3, 1);
      pointer-events: none;
    `;
    document.body.appendChild(modal);
  }

  const emp = findEmployeeByCode(data.employee_code) || {};
  const name = data.employee_name || emp.full_name || 'Nhân viên';
  const code = data.employee_code || '';
  const dept = data.department || emp.department_id || (window._lang === 'vi' ? 'Chưa xếp phòng' : 'Unassigned');
  const avatar = data.avatar_url || emp.avatar_url || '';
  const verifyMode = data.verify_mode || 'Vân tay (SDK)';
  const checkTime = data.check_time || new Date().toLocaleTimeString();
  const deviceName = data.device_name || 'Máy chấm công';
  
  const initials = getEmployeeInitials(name);

  modal.innerHTML = `
    <div style="width: 52px; height: 52px; border-radius: 50%; background: linear-gradient(135deg, #10b981, #059669); padding: 2px; flex-shrink: 0; position: relative;">
      ${avatar 
        ? `<img src="${avatar}" style="width: 100%; height: 100%; border-radius: 50%; object-fit: cover;">`
        : `<div style="width: 100%; height: 100%; border-radius: 50%; background: #1e293b; display: flex; align-items: center; justify-content: center; font-weight: bold; font-size: 18px; color: #10b981;">${initials}</div>`
      }
      <div style="position: absolute; bottom: 0; right: 0; width: 14px; height: 14px; background: #10b981; border: 2px solid #0f172a; border-radius: 50%;"></div>
    </div>
    <div style="flex: 1; min-width: 0;">
      <div style="font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.5px; color: #34d399; margin-bottom: 2px; display: flex; align-items: center; gap: 6px;">
        <span>⚡ REALTIME SCAN CONFIRMED</span>
      </div>
      <div style="font-size: 16px; font-weight: 700; color: #f8fafc; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
        ${name} <span style="font-size: 13px; font-weight: 500; color: #94a3b8;">(${code})</span>
      </div>
      <div style="font-size: 12px; color: #cbd5e1; margin-top: 2px; display: flex; gap: 8px; flex-wrap: wrap;">
        <span>🏢 ${dept}</span>
        <span>🕒 ${checkTime}</span>
      </div>
    </div>
  `;

  requestAnimationFrame(() => {
    modal.style.opacity = '1';
    modal.style.transform = 'translateX(-50%) translateY(0)';
  });

  if (window._realtimeToastTimer) clearTimeout(window._realtimeToastTimer);
  window._realtimeToastTimer = setTimeout(() => {
    modal.style.opacity = '0';
    modal.style.transform = 'translateX(-50%) translateY(-20px)';
  }, 4500);
}

document.addEventListener('DOMContentLoaded', () => {
  bindElements();
  bindEvents();
  if (state.token) {
    showApp();
    loadAll();
    connectSSE();
    populateStopBatchDeviceSelect();
    startAttendanceAutoRefresh();
  } else {
    showLogin();
  }
});

function bindElements() {
  els.loginView = document.getElementById('loginView');
  els.mainApp = document.getElementById('mainApp');
  els.loginForm = document.getElementById('loginForm');
  els.username = document.getElementById('username');
  els.password = document.getElementById('password');
  els.loginMessage = document.getElementById('loginMessage');
  els.currentUser = document.getElementById('currentUser');
  els.pageTitle = document.getElementById('pageTitle');
  els.refreshBtn = document.getElementById('refreshBtn');
  els.logoutBtn = document.getElementById('logoutBtn');
  els.deviceForm = document.getElementById('deviceForm');
  els.deviceId = document.getElementById('deviceId');
  els.deviceName = document.getElementById('deviceName');
  els.deviceType = document.getElementById('deviceType');
  els.deviceIp = document.getElementById('deviceIp');
  els.devicePort = document.getElementById('devicePort');
  els.deviceSerial = document.getElementById('deviceSerial');
  els.deviceSerialADMS = document.getElementById('deviceSerialADMS');
  els.deviceADMSEnabled = document.getElementById('deviceADMSEnabled');
  els.deviceFirmware = document.getElementById('deviceFirmware');
  els.deviceMac = document.getElementById('deviceMac');
  els.deviceLocation = document.getElementById('deviceLocation');
  els.deviceMessage = document.getElementById('deviceMessage');
  els.deviceTableBody = document.getElementById('deviceTableBody');
  els.cancelDeviceBtn = document.getElementById('cancelDeviceBtn');
  els.employeeForm = document.getElementById('employeeForm');
  els.employeeId = document.getElementById('employeeId');
  els.employeeCode = document.getElementById('employeeCode');
  els.employeeName = document.getElementById('employeeName');
  els.employeeJobTitle = document.getElementById('employeeJobTitle');
  els.employeeDepartment = document.getElementById('employeeDepartment');
  els.employeeCard = document.getElementById('employeeCard');
  els.employeeEmail = document.getElementById('employeeEmail');
  els.employeePhone = document.getElementById('employeePhone');
  els.employeeZaloUserId = document.getElementById('employeeZaloUserId');
  els.employeeGender = document.getElementById('employeeGender');
  els.employeeDob = document.getElementById('employeeDob');
  els.employeeJoinDate = document.getElementById('employeeJoinDate');
  els.employeeAvatar = document.getElementById('employeeAvatar');
  els.employeeEnrollFingerprint = document.getElementById('employeeEnrollFingerprint');
  els.employeeEnrollSection = document.getElementById('employeeEnrollSection');
  els.employeeEnrollDeviceId = document.getElementById('employeeEnrollDeviceId');
  els.employeeEnrollDeviceUserId = document.getElementById('employeeEnrollDeviceUserId');
  els.employeeStatus = document.getElementById('employeeStatus');
  els.employeeMessage = document.getElementById('employeeMessage');
  els.employeeTableBody = document.getElementById('employeeTableBody');
  els.cancelEmployeeBtn = document.getElementById('cancelEmployeeBtn');
  els.employeeFileInput = document.getElementById('employeeFileInput');
  els.importEmployeesBtn = document.getElementById('importEmployeesBtn');
  els.attendanceFrom = document.getElementById('attendanceFrom');
  els.attendanceTo = document.getElementById('attendanceTo');
  els.attendanceCode = document.getElementById('attendanceCode');
  els.attendanceTableBody = document.getElementById('attendanceTableBody');
  els.attendanceSummaryBody = document.getElementById('attendanceSummaryBody');
  els.loadAttendanceBtn = document.getElementById('loadAttendanceBtn');
  els.applyAttendanceBtn = document.getElementById('applyAttendanceBtn');
  els.syncTableBody = document.getElementById('syncTableBody');
  els.syncNowBtn = document.getElementById('syncNowBtn');
  els.syncMessage = document.getElementById('syncMessage');
  els.recentDevices = document.getElementById('recentDevices');
  els.recentSync = document.getElementById('recentSync');
  els.deviceCount = document.getElementById('deviceCount');
  els.employeeCount = document.getElementById('employeeCount');
  els.attendanceCount = document.getElementById('attendanceCount');
  els.syncCount = document.getElementById('syncCount');
  els.lastScanCard = document.getElementById('lastScanCard');
  els.lastScanAvatar = document.getElementById('lastScanAvatar');
  els.lastScanLabel = document.getElementById('lastScanLabel');
  els.lastScanName = document.getElementById('lastScanName');
  els.lastScanCode = document.getElementById('lastScanCode');
  els.lastScanRole = document.getElementById('lastScanRole');
  els.lastScanContact = document.getElementById('lastScanContact');
  els.lastScanType = document.getElementById('lastScanType');
  els.lastScanTime = document.getElementById('lastScanTime');
  els.lastScanDevice = document.getElementById('lastScanDevice');

  // Shifts elements
  els.shiftsView = document.getElementById('shiftsView');
  els.shiftForm = document.getElementById('shiftForm');
  els.shiftName = document.getElementById('shiftName');
  els.shiftStartTime = document.getElementById('shiftStartTime');
  els.shiftEndTime = document.getElementById('shiftEndTime');
  els.shiftBreakMinutes = document.getElementById('shiftBreakMinutes');
  els.shiftLateGrace = document.getElementById('shiftLateGrace');
  els.shiftEarlyGrace = document.getElementById('shiftEarlyGrace');
  els.shiftMessage = document.getElementById('shiftMessage');
  els.shiftTableBody = document.getElementById('shiftTableBody');
  els.assignShiftForm = document.getElementById('assignShiftForm');
  els.assignEmployeeId = document.getElementById('assignEmployeeId');
  els.assignShiftId = document.getElementById('assignShiftId');
  els.assignStartDate = document.getElementById('assignStartDate');
  els.assignMessage = document.getElementById('assignMessage');
  els.processAttendanceBtn = document.getElementById('processAttendanceBtn');
  els.attendanceMessage = document.getElementById('attendanceMessage');
  els.attendanceSummarySection = document.getElementById('attendanceSummarySection');
  els.attendanceRawSection = document.getElementById('attendanceRawSection');
  els.showAttendanceSummaryBtn = document.getElementById('showAttendanceSummaryBtn');
  els.showAttendanceRawBtn = document.getElementById('showAttendanceRawBtn');

  // Reports elements
  els.reportsView = document.getElementById('reportsView');
  els.reportMonth = document.getElementById('reportMonth');
  els.applyReportBtn = document.getElementById('applyReportBtn');
  els.reportTableHeader = document.getElementById('reportTableHeader');
  els.reportTableBody = document.getElementById('reportTableBody');
  els.exportExcelBtn = document.getElementById('exportExcelBtn');
  els.sendMonthlyReportBtn = document.getElementById('sendMonthlyReportBtn');
  els.reportMessage = document.getElementById('reportMessage');

  // Audit logs elements
  els.auditView = document.getElementById('auditView');
  els.refreshAuditBtn = document.getElementById('refreshAuditBtn');
  els.auditTableBody = document.getElementById('auditTableBody');

  // Modals & Search box bindings
  els.openNewDeviceModalBtn = document.getElementById('openNewDeviceModalBtn');
  els.closeDeviceModalBtn = document.getElementById('closeDeviceModalBtn');
  els.deviceModal = document.getElementById('deviceModal');
  els.deviceModalTitle = document.getElementById('deviceModalTitle');
  els.deviceSearch = document.getElementById('deviceSearch');
  els.syncDeviceSelect = document.getElementById('syncDeviceSelect');
  els.pushAllNowBtn = document.getElementById('pushAllNowBtn');
  els.pullFromDeviceNowBtn = document.getElementById('pullFromDeviceNowBtn');
  els.backupDeviceModal = document.getElementById('backupDeviceModal');
  els.closeBackupDeviceModalBtn = document.getElementById('closeBackupDeviceModalBtn');
  els.backupCancelBtn = document.getElementById('backupCancelBtn');
  els.backupSubmitBtn = document.getElementById('backupSubmitBtn');
  els.backupSourceDeviceName = document.getElementById('backupSourceDeviceName');
  els.backupSourceDeviceId = document.getElementById('backupSourceDeviceId');
  els.backupTargetDevicesList = document.getElementById('backupTargetDevicesList');
  els.toolbarBackupSourceSelect = document.getElementById('toolbarBackupSourceSelect');
  els.toolbarBackupTargetSelect = document.getElementById('toolbarBackupTargetSelect');
  els.toolbarBackupBtn = document.getElementById('toolbarBackupBtn');

  els.openNewEmployeeModalBtn = document.getElementById('openNewEmployeeModalBtn');
  els.deleteAllEmployeesBtn = document.getElementById('deleteAllEmployeesBtn');
  els.closeEmployeeModalBtn = document.getElementById('closeEmployeeModalBtn');
  els.employeeModal = document.getElementById('employeeModal');
  els.employeeModalTitle = document.getElementById('employeeModalTitle');
  els.employeeSearch = document.getElementById('employeeSearch');

  els.openNewShiftModalBtn = document.getElementById('openNewShiftModalBtn');
  els.closeShiftModalBtn = document.getElementById('closeShiftModalBtn');
  els.closeShiftModalCancelBtn = document.getElementById('closeShiftModalCancelBtn');
  els.shiftModal = document.getElementById('shiftModal');

  els.openAssignShiftModalBtn = document.getElementById('openAssignShiftModalBtn');
  els.assignType = document.getElementById('assignType');
  els.assignShiftIdLabel = document.getElementById('assignShiftIdLabel');
  els.assignShiftId = document.getElementById('assignShiftId');
  els.assignRotationPatternIdLabel = document.getElementById('assignRotationPatternIdLabel');
  els.assignRotationPatternId = document.getElementById('assignRotationPatternId');
  els.assignEndDate = document.getElementById('assignEndDate');

  // Batch Assign Elements
  els.openBatchAssignModalBtn = document.getElementById('openBatchAssignModalBtn');
  els.batchAssignModal = document.getElementById('batchAssignModal');
  els.closeBatchAssignModalBtn = document.getElementById('closeBatchAssignModalBtn');
  els.closeBatchAssignModalCancelBtn = document.getElementById('closeBatchAssignModalCancelBtn');
  els.batchAssignForm = document.getElementById('batchAssignForm');
  els.batchAssignTargetType = document.getElementById('batchAssignTargetType');
  els.batchAssignDeptLabel = document.getElementById('batchAssignDeptLabel');
  els.batchAssignDeptSelect = document.getElementById('batchAssignDeptSelect');
  els.batchAssignEmployeesLabel = document.getElementById('batchAssignEmployeesLabel');
  els.batchAssignEmployeesSelect = document.getElementById('batchAssignEmployeesSelect');
  els.batchAssignType = document.getElementById('batchAssignType');
  els.batchAssignShiftIdLabel = document.getElementById('batchAssignShiftIdLabel');
  els.batchAssignShiftId = document.getElementById('batchAssignShiftId');
  els.batchAssignRotationPatternIdLabel = document.getElementById('batchAssignRotationPatternIdLabel');
  els.batchAssignRotationPatternId = document.getElementById('batchAssignRotationPatternId');
  els.batchAssignStartDate = document.getElementById('batchAssignStartDate');
  els.batchAssignEndDate = document.getElementById('batchAssignEndDate');

  // Rotation Pattern Elements
  els.openNewRotationModalBtn = document.getElementById('openNewRotationModalBtn');
  els.rotationModal = document.getElementById('rotationModal');
  els.closeRotationModalBtn = document.getElementById('closeRotationModalBtn');
  els.closeRotationModalCancelBtn = document.getElementById('closeRotationModalCancelBtn');
  els.rotationForm = document.getElementById('rotationForm');
  els.rotationName = document.getElementById('rotationName');
  els.rotationStepsContainer = document.getElementById('rotationStepsContainer');
  els.addRotationStepBtn = document.getElementById('addRotationStepBtn');
  els.rotationTableBody = document.getElementById('rotationTableBody');
  els.rotationMessage = document.getElementById('rotationMessage');

  // Shift Filter / List Elements
  els.shiftFilterTabs = document.getElementById('shiftFilterTabs');
  els.employeeShiftTableBody = document.getElementById('employeeShiftTableBody');

  // Shift Swap Elements
  els.openShiftSwapModalBtn = document.getElementById('openShiftSwapModalBtn');
  els.shiftSwapModal = document.getElementById('shiftSwapModal');
  els.closeShiftSwapModalBtn = document.getElementById('closeShiftSwapModalBtn');
  els.closeShiftSwapModalCancelBtn = document.getElementById('closeShiftSwapModalCancelBtn');
  els.shiftSwapForm = document.getElementById('shiftSwapForm');
  els.swapRequesterEmployeeId = document.getElementById('swapRequesterEmployeeId');
  els.swapRequesterDate = document.getElementById('swapRequesterDate');
  els.swapTargetEmployeeId = document.getElementById('swapTargetEmployeeId');
  els.swapTargetDate = document.getElementById('swapTargetDate');
  els.shiftSwapTableBody = document.getElementById('shiftSwapTableBody');
  els.shiftSwapMessage = document.getElementById('shiftSwapMessage');
  els.closeAssignShiftModalBtn = document.getElementById('closeAssignShiftModalBtn');
  els.closeAssignShiftModalCancelBtn = document.getElementById('closeAssignShiftModalCancelBtn');
  els.assignShiftModal = document.getElementById('assignShiftModal');

  // ESS Elements
  els.essView = document.getElementById('essView');
  els.essForm = document.getElementById('essForm');
  els.essEmployeeId = document.getElementById('essEmployeeId');
  els.essType = document.getElementById('essType');
  els.essLeaveFields = document.getElementById('essLeaveFields');
  els.essLeaveType = document.getElementById('essLeaveType');
  els.essLeaveStartDate = document.getElementById('essLeaveStartDate');
  els.essLeaveEndDate = document.getElementById('essLeaveEndDate');
  els.essOvertimeFields = document.getElementById('essOvertimeFields');
  els.essOtDate = document.getElementById('essOtDate');
  els.essOtStartTime = document.getElementById('essOtStartTime');
  els.essOtEndTime = document.getElementById('essOtEndTime');
  els.essCorrectionFields = document.getElementById('essCorrectionFields');
  els.essCorrDate = document.getElementById('essCorrDate');
  els.essCorrCheckType = document.getElementById('essCorrCheckType');
  els.essCorrTime = document.getElementById('essCorrTime');
  els.essReason = document.getElementById('essReason');
  els.essMessage = document.getElementById('essMessage');
  els.essTableBody = document.getElementById('essTableBody');
  els.essFilterType = document.getElementById('essFilterType');
  els.essFilterStatus = document.getElementById('essFilterStatus');
  els.openNewEssModalBtn = document.getElementById('openNewEssModalBtn');
  els.closeEssModalBtn = document.getElementById('closeEssModalBtn');
  els.closeEssModalCancelBtn = document.getElementById('closeEssModalCancelBtn');
  els.essModal = document.getElementById('essModal');

  // Fingerprint Modal Elements
  els.fingerprintModal = document.getElementById('fingerprintModal');
  els.closeFingerprintModalBtn = document.getElementById('closeFingerprintModalBtn');
  els.fpModalEmployeeName = document.getElementById('fpModalEmployeeName');
  els.fpModalEmployeeCode = document.getElementById('fpModalEmployeeCode');
  els.fpModalStatusBadge = document.getElementById('fpModalStatusBadge');
  els.fpEnrollDeviceId = document.getElementById('fpEnrollDeviceId');
  els.fpEnrollFingerIndex = document.getElementById('fpEnrollFingerIndex');
  els.fpTriggerEnrollBtn = document.getElementById('fpTriggerEnrollBtn');
  els.fpEnrollSection = document.getElementById('fpEnrollSection');
  els.fpInstructionBox = document.getElementById('fpInstructionBox');
  els.fpFingerprintList = document.getElementById('fpFingerprintList');
  els.fpFingerprintListItems = document.getElementById('fpFingerprintListItems');
  els.fpActionsSection = document.getElementById('fpActionsSection');
  els.fpReEnrollBtn = document.getElementById('fpReEnrollBtn');
  els.fpPushToDevicesBtn = document.getElementById('fpPushToDevicesBtn');
  els.fpDeleteBtn = document.getElementById('fpDeleteBtn');
  els.fpManualConfirmLink = document.getElementById('fpManualConfirmLink');

  // Sync Single Employee Modal Elements
  els.syncNowBtn = document.getElementById('syncNowBtn');
  els.syncMessage = document.getElementById('syncMessage');
  els.recentDevices = document.getElementById('recentDevices');
  els.recentSync = document.getElementById('recentSync');
  els.deviceCount = document.getElementById('deviceCount');
  els.employeeCount = document.getElementById('employeeCount');
  els.attendanceCount = document.getElementById('attendanceCount');
  els.syncCount = document.getElementById('syncCount');
  els.lastScanCard = document.getElementById('lastScanCard');
  els.lastScanAvatar = document.getElementById('lastScanAvatar');
  els.lastScanLabel = document.getElementById('lastScanLabel');
  els.lastScanName = document.getElementById('lastScanName');
  els.lastScanCode = document.getElementById('lastScanCode');
  els.lastScanRole = document.getElementById('lastScanRole');
  els.lastScanContact = document.getElementById('lastScanContact');
  els.lastScanType = document.getElementById('lastScanType');
  els.lastScanTime = document.getElementById('lastScanTime');
  els.lastScanDevice = document.getElementById('lastScanDevice');

  // Shifts elements
  els.shiftsView = document.getElementById('shiftsView');
  els.shiftForm = document.getElementById('shiftForm');
  els.shiftName = document.getElementById('shiftName');
  els.shiftStartTime = document.getElementById('shiftStartTime');
  els.shiftEndTime = document.getElementById('shiftEndTime');
  els.shiftBreakMinutes = document.getElementById('shiftBreakMinutes');
  els.shiftLateGrace = document.getElementById('shiftLateGrace');
  els.shiftEarlyGrace = document.getElementById('shiftEarlyGrace');
  els.shiftMessage = document.getElementById('shiftMessage');
  els.shiftTableBody = document.getElementById('shiftTableBody');
  els.assignShiftForm = document.getElementById('assignShiftForm');
  els.assignEmployeeId = document.getElementById('assignEmployeeId');
  els.assignShiftId = document.getElementById('assignShiftId');
  els.assignStartDate = document.getElementById('assignStartDate');
  els.assignMessage = document.getElementById('assignMessage');
  els.processAttendanceBtn = document.getElementById('processAttendanceBtn');
  els.attendanceMessage = document.getElementById('attendanceMessage');
  els.attendanceSummarySection = document.getElementById('attendanceSummarySection');
  els.attendanceRawSection = document.getElementById('attendanceRawSection');
  els.showAttendanceSummaryBtn = document.getElementById('showAttendanceSummaryBtn');
  els.showAttendanceRawBtn = document.getElementById('showAttendanceRawBtn');

  // Reports elements
  els.reportsView = document.getElementById('reportsView');
  els.reportMonth = document.getElementById('reportMonth');
  els.applyReportBtn = document.getElementById('applyReportBtn');
  els.reportTableHeader = document.getElementById('reportTableHeader');
  els.reportTableBody = document.getElementById('reportTableBody');
  els.exportExcelBtn = document.getElementById('exportExcelBtn');
  els.sendMonthlyReportBtn = document.getElementById('sendMonthlyReportBtn');
  els.reportMessage = document.getElementById('reportMessage');

  // Audit logs elements
  els.auditView = document.getElementById('auditView');
  els.refreshAuditBtn = document.getElementById('refreshAuditBtn');
  els.auditTableBody = document.getElementById('auditTableBody');

  // Modals & Search box bindings
  els.openNewDeviceModalBtn = document.getElementById('openNewDeviceModalBtn');
  els.closeDeviceModalBtn = document.getElementById('closeDeviceModalBtn');
  els.deviceModal = document.getElementById('deviceModal');
  els.deviceModalTitle = document.getElementById('deviceModalTitle');
  els.deviceSearch = document.getElementById('deviceSearch');
  els.syncDeviceSelect = document.getElementById('syncDeviceSelect');
  els.pushAllNowBtn = document.getElementById('pushAllNowBtn');
  els.pullFromDeviceNowBtn = document.getElementById('pullFromDeviceNowBtn');
  els.backupDeviceModal = document.getElementById('backupDeviceModal');
  els.closeBackupDeviceModalBtn = document.getElementById('closeBackupDeviceModalBtn');
  els.backupCancelBtn = document.getElementById('backupCancelBtn');
  els.backupSubmitBtn = document.getElementById('backupSubmitBtn');
  els.backupSourceDeviceName = document.getElementById('backupSourceDeviceName');
  els.backupSourceDeviceId = document.getElementById('backupSourceDeviceId');
  els.backupTargetDevicesList = document.getElementById('backupTargetDevicesList');
  els.toolbarBackupSourceSelect = document.getElementById('toolbarBackupSourceSelect');
  els.toolbarBackupTargetSelect = document.getElementById('toolbarBackupTargetSelect');
  els.toolbarBackupBtn = document.getElementById('toolbarBackupBtn');

  els.openNewEmployeeModalBtn = document.getElementById('openNewEmployeeModalBtn');
  els.deleteAllEmployeesBtn = document.getElementById('deleteAllEmployeesBtn');
  els.closeEmployeeModalBtn = document.getElementById('closeEmployeeModalBtn');
  els.employeeModal = document.getElementById('employeeModal');
  els.employeeModalTitle = document.getElementById('employeeModalTitle');
  els.employeeSearch = document.getElementById('employeeSearch');

  els.openNewShiftModalBtn = document.getElementById('openNewShiftModalBtn');
  els.closeShiftModalBtn = document.getElementById('closeShiftModalBtn');
  els.closeShiftModalCancelBtn = document.getElementById('closeShiftModalCancelBtn');
  els.shiftModal = document.getElementById('shiftModal');

  els.openAssignShiftModalBtn = document.getElementById('openAssignShiftModalBtn');
  els.assignType = document.getElementById('assignType');
  els.assignShiftIdLabel = document.getElementById('assignShiftIdLabel');
  els.assignShiftId = document.getElementById('assignShiftId');
  els.assignRotationPatternIdLabel = document.getElementById('assignRotationPatternIdLabel');
  els.assignRotationPatternId = document.getElementById('assignRotationPatternId');
  els.assignEndDate = document.getElementById('assignEndDate');

  // Batch Assign Elements
  els.openBatchAssignModalBtn = document.getElementById('openBatchAssignModalBtn');
  els.batchAssignModal = document.getElementById('batchAssignModal');
  els.closeBatchAssignModalBtn = document.getElementById('closeBatchAssignModalBtn');
  els.closeBatchAssignModalCancelBtn = document.getElementById('closeBatchAssignModalCancelBtn');
  els.batchAssignForm = document.getElementById('batchAssignForm');
  els.batchAssignTargetType = document.getElementById('batchAssignTargetType');
  els.batchAssignDeptLabel = document.getElementById('batchAssignDeptLabel');
  els.batchAssignDeptSelect = document.getElementById('batchAssignDeptSelect');
  els.batchAssignEmployeesLabel = document.getElementById('batchAssignEmployeesLabel');
  els.batchAssignEmployeesSelect = document.getElementById('batchAssignEmployeesSelect');
  els.batchAssignType = document.getElementById('batchAssignType');
  els.batchAssignShiftIdLabel = document.getElementById('batchAssignShiftIdLabel');
  els.batchAssignShiftId = document.getElementById('batchAssignShiftId');
  els.batchAssignRotationPatternIdLabel = document.getElementById('batchAssignRotationPatternIdLabel');
  els.batchAssignRotationPatternId = document.getElementById('batchAssignRotationPatternId');
  els.batchAssignStartDate = document.getElementById('batchAssignStartDate');
  els.batchAssignEndDate = document.getElementById('batchAssignEndDate');

  // Rotation Pattern Elements
  els.openNewRotationModalBtn = document.getElementById('openNewRotationModalBtn');
  els.rotationModal = document.getElementById('rotationModal');
  els.closeRotationModalBtn = document.getElementById('closeRotationModalBtn');
  els.closeRotationModalCancelBtn = document.getElementById('closeRotationModalCancelBtn');
  els.rotationForm = document.getElementById('rotationForm');
  els.rotationName = document.getElementById('rotationName');
  els.rotationStepsContainer = document.getElementById('rotationStepsContainer');
  els.addRotationStepBtn = document.getElementById('addRotationStepBtn');
  els.rotationTableBody = document.getElementById('rotationTableBody');
  els.rotationMessage = document.getElementById('rotationMessage');

  // Shift Filter / List Elements
  els.shiftFilterTabs = document.getElementById('shiftFilterTabs');
  els.employeeShiftTableBody = document.getElementById('employeeShiftTableBody');

  // Shift Swap Elements
  els.openShiftSwapModalBtn = document.getElementById('openShiftSwapModalBtn');
  els.shiftSwapModal = document.getElementById('shiftSwapModal');
  els.closeShiftSwapModalBtn = document.getElementById('closeShiftSwapModalBtn');
  els.closeShiftSwapModalCancelBtn = document.getElementById('closeShiftSwapModalCancelBtn');
  els.shiftSwapForm = document.getElementById('shiftSwapForm');
  els.swapRequesterEmployeeId = document.getElementById('swapRequesterEmployeeId');
  els.swapRequesterDate = document.getElementById('swapRequesterDate');
  els.swapTargetEmployeeId = document.getElementById('swapTargetEmployeeId');
  els.swapTargetDate = document.getElementById('swapTargetDate');
  els.shiftSwapTableBody = document.getElementById('shiftSwapTableBody');
  els.shiftSwapMessage = document.getElementById('shiftSwapMessage');
  els.closeAssignShiftModalBtn = document.getElementById('closeAssignShiftModalBtn');
  els.closeAssignShiftModalCancelBtn = document.getElementById('closeAssignShiftModalCancelBtn');
  els.assignShiftModal = document.getElementById('assignShiftModal');

  // ESS Elements
  els.essView = document.getElementById('essView');
  els.essForm = document.getElementById('essForm');
  els.essEmployeeId = document.getElementById('essEmployeeId');
  els.essType = document.getElementById('essType');
  els.essLeaveFields = document.getElementById('essLeaveFields');
  els.essLeaveType = document.getElementById('essLeaveType');
  els.essLeaveStartDate = document.getElementById('essLeaveStartDate');
  els.essLeaveEndDate = document.getElementById('essLeaveEndDate');
  els.essOvertimeFields = document.getElementById('essOvertimeFields');
  els.essOtDate = document.getElementById('essOtDate');
  els.essOtStartTime = document.getElementById('essOtStartTime');
  els.essOtEndTime = document.getElementById('essOtEndTime');
  els.essCorrectionFields = document.getElementById('essCorrectionFields');
  els.essCorrDate = document.getElementById('essCorrDate');
  els.essCorrCheckType = document.getElementById('essCorrCheckType');
  els.essCorrTime = document.getElementById('essCorrTime');
  els.essReason = document.getElementById('essReason');
  els.essMessage = document.getElementById('essMessage');
  els.essTableBody = document.getElementById('essTableBody');
  els.essFilterType = document.getElementById('essFilterType');
  els.essFilterStatus = document.getElementById('essFilterStatus');
  els.openNewEssModalBtn = document.getElementById('openNewEssModalBtn');
  els.closeEssModalBtn = document.getElementById('closeEssModalBtn');
  els.closeEssModalCancelBtn = document.getElementById('closeEssModalCancelBtn');
  els.essModal = document.getElementById('essModal');

  // Fingerprint Modal Elements
  els.fingerprintModal = document.getElementById('fingerprintModal');
  els.closeFingerprintModalBtn = document.getElementById('closeFingerprintModalBtn');
  els.fpModalEmployeeName = document.getElementById('fpModalEmployeeName');
  els.fpModalEmployeeCode = document.getElementById('fpModalEmployeeCode');
  els.fpModalStatusBadge = document.getElementById('fpModalStatusBadge');
  els.fpEnrollDeviceId = document.getElementById('fpEnrollDeviceId');
  els.fpEnrollFingerIndex = document.getElementById('fpEnrollFingerIndex');
  els.fpTriggerEnrollBtn = document.getElementById('fpTriggerEnrollBtn');
  els.fpEnrollSection = document.getElementById('fpEnrollSection');
  els.fpInstructionBox = document.getElementById('fpInstructionBox');
  els.fpFingerprintList = document.getElementById('fpFingerprintList');
  els.fpFingerprintListItems = document.getElementById('fpFingerprintListItems');
  els.fpActionsSection = document.getElementById('fpActionsSection');
  els.fpReEnrollBtn = document.getElementById('fpReEnrollBtn');
  els.fpPushToDevicesBtn = document.getElementById('fpPushToDevicesBtn');
  els.fpDeleteBtn = document.getElementById('fpDeleteBtn');
  els.fpManualConfirmLink = document.getElementById('fpManualConfirmLink');

  // Sync Single Employee Modal Elements
  els.syncSingleEmployeeModal = document.getElementById('syncSingleEmployeeModal');
  els.syncSingleEmployeeEmpName = document.getElementById('syncSingleEmployeeEmpName');
  els.syncSingleEmployeeEmpId = document.getElementById('syncSingleEmployeeEmpId');
  els.syncSingleEmployeeDeviceSelect = document.getElementById('syncSingleEmployeeDeviceSelect');
  els.syncSingleEmployeeDeviceUserId = document.getElementById('syncSingleEmployeeDeviceUserId');
  els.syncSingleEmployeeStatusMsg = document.getElementById('syncSingleEmployeeStatusMsg');
  els.closeSyncSingleEmployeeModalBtn = document.getElementById('closeSyncSingleEmployeeModalBtn');
  els.cancelSyncSingleEmployeeBtn = document.getElementById('cancelSyncSingleEmployeeBtn');
  els.startCameraButton = document.getElementById('startCameraButton');
  els.stopCameraButton = document.getElementById('stopCameraButton');
  els.registerFaceModal = document.getElementById('registerFaceModal');
  els.closeRegisterFaceModalBtn = document.getElementById('closeRegisterFaceModalBtn');
  els.startRegisterFaceCamBtn = document.getElementById('startRegisterFaceCamBtn');
  els.captureFaceBtn = document.getElementById('captureFaceBtn');
  els.openFaceScanModalBtn = document.getElementById('openFaceScanModalBtn');
  els.faceScanModal = document.getElementById('faceScanModal');
  els.closeFaceScanModalBtn = document.getElementById('closeFaceScanModalBtn');
}

function bindEvents() {
  if (els.openFaceScanModalBtn) {
    els.openFaceScanModalBtn.addEventListener('click', () => {
      if (els.faceScanModal) els.faceScanModal.classList.add('active');
      document.body.classList.add('modal-open');
      startCameraAttendance();
    });
  }
  if (els.closeFaceScanModalBtn) {
    els.closeFaceScanModalBtn.addEventListener('click', () => {
      stopCameraAttendance();
      if (els.faceScanModal) els.faceScanModal.classList.remove('active');
      document.body.classList.remove('modal-open');
    });
  }
  els.loginForm.addEventListener('submit', onLogin);
  els.logoutBtn.addEventListener('click', logout);
  els.refreshBtn.addEventListener('click', loadAll);
  document.querySelectorAll('.nav-item').forEach((btn) => {
    btn.addEventListener('click', () => switchView(btn.dataset.view));
  });
  document.querySelectorAll('[data-view]').forEach((btn) => {
    btn.addEventListener('click', () => switchView(btn.dataset.view));
  });
  els.openNewDeviceModalBtn.addEventListener('click', () => {
    console.log('[debug] open new device modal button clicked');
    showToast(window._lang === 'vi' ? 'Đã mở form thiết bị' : 'Device form opened', 'info');
    resetDeviceForm();
    els.deviceModalTitle.textContent = window._lang === 'vi' ? 'Thêm thiết bị mới' : 'Add New Device';
    els.deviceModal.classList.add('active');
  });
  els.closeDeviceModalBtn.addEventListener('click', () => els.deviceModal.classList.remove('active'));
  els.cancelDeviceBtn.addEventListener('click', () => els.deviceModal.classList.remove('active'));
  els.closeBackupDeviceModalBtn.addEventListener('click', () => els.backupDeviceModal.classList.remove('active'));
  els.backupCancelBtn.addEventListener('click', () => els.backupDeviceModal.classList.remove('active'));
  els.backupSubmitBtn.addEventListener('click', async (event) => {
    showToast(window._lang === 'vi' ? 'Đang sao chép dữ liệu giữa các máy...' : 'Copying data between devices...', 'info');
    await onSubmitBackup(event);
  });
  if (els.toolbarBackupBtn) els.toolbarBackupBtn.addEventListener('click', runToolbarBackup);
  els.deviceForm.addEventListener('submit', onSaveDevice);
  els.openNewEmployeeModalBtn.addEventListener('click', () => {
    console.log('[debug] open new employee modal button clicked');
    showToast(window._lang === 'vi' ? 'Đã mở form nhân viên' : 'Employee form opened', 'info');
    resetEmployeeForm();
    updateEmployeeEnrollDeviceOptions();
    els.employeeModalTitle.textContent = window._lang === 'vi' ? 'Thêm nhân viên mới' : 'Add New Employee';
    els.employeeModal.classList.add('active');
  });
  if (els.deleteAllEmployeesBtn) {
    els.deleteAllEmployeesBtn.addEventListener('click', confirmDeleteAllEmployees);
  }
  els.closeEmployeeModalBtn.addEventListener('click', () => els.employeeModal.classList.remove('active'));
  els.cancelEmployeeBtn.addEventListener('click', () => els.employeeModal.classList.remove('active'));
  if (els.employeeEnrollFingerprint) {
    els.employeeEnrollFingerprint.addEventListener('change', toggleEmployeeEnrollFields);
  }
  if (els.employeeCode) {
    els.employeeCode.addEventListener('input', () => {
      if (els.employeeEnrollDeviceUserId && !els.employeeEnrollDeviceUserId.value) {
        els.employeeEnrollDeviceUserId.value = els.employeeCode.value;
      }
    });
  }
  els.employeeForm.addEventListener('submit', (event) => {
    console.log('[debug] employeeForm submit attached');
    showToast(window._lang === 'vi' ? 'Đang lưu nhân viên...' : 'Saving employee...', 'info');
    onSaveEmployee(event);
  });
  els.openNewShiftModalBtn.addEventListener('click', () => {
    state.editingShiftId = null;
    els.shiftForm.reset();
    const headerTitle = els.shiftModal.querySelector('.modal-header h3');
    if (headerTitle) headerTitle.textContent = t('modalAddShift');
    const submitBtn = els.shiftForm.querySelector('button[type="submit"]');
    if (submitBtn) submitBtn.textContent = t('btnCreateShift');
    els.shiftModal.classList.add('active');
  });
  els.closeShiftModalBtn.addEventListener('click', () => els.shiftModal.classList.remove('active'));
  els.closeShiftModalCancelBtn.addEventListener('click', () => els.shiftModal.classList.remove('active'));
  els.shiftForm.addEventListener('submit', onSaveShift);
  els.openAssignShiftModalBtn.addEventListener('click', () => {
    els.assignShiftForm.reset();
    if (els.assignType) els.assignType.value = "shift";
    toggleAssignFields();
    els.assignShiftModal.classList.add('active');
  });
  els.assignType.addEventListener('change', toggleAssignFields);

  // Batch Assign Event Listeners
  if (els.openBatchAssignModalBtn) {
    els.openBatchAssignModalBtn.addEventListener('click', () => {
      els.batchAssignForm.reset();
      if (els.batchAssignTargetType) els.batchAssignTargetType.value = "all";
      if (els.batchAssignType) els.batchAssignType.value = "shift";
      toggleBatchAssignFields();
      els.batchAssignModal.classList.add('active');
    });
    els.closeBatchAssignModalBtn.addEventListener('click', () => els.batchAssignModal.classList.remove('active'));
    els.closeBatchAssignModalCancelBtn.addEventListener('click', () => els.batchAssignModal.classList.remove('active'));
    els.batchAssignForm.addEventListener('submit', onBatchAssignShift);
    els.batchAssignTargetType.addEventListener('change', toggleBatchAssignFields);
    els.batchAssignType.addEventListener('change', toggleBatchAssignFields);
  }

  // Rotation Pattern Event Listeners
  if (els.openNewRotationModalBtn) {
    els.openNewRotationModalBtn.addEventListener('click', () => {
      state.editingRotationId = null;
      els.rotationForm.reset();
      els.rotationStepsContainer.innerHTML = '';
      addRotationStep(); // Start with one step
      const headerTitle = els.rotationModal.querySelector('.modal-header h3');
      if (headerTitle) headerTitle.textContent = t('createRotationPatternTitle');
      const submitBtn = els.rotationForm.querySelector('button[type="submit"]');
      if (submitBtn) submitBtn.textContent = t('btnSave') || 'Lưu chu kỳ';
      els.rotationModal.classList.add('active');
    });
    els.closeRotationModalBtn.addEventListener('click', () => els.rotationModal.classList.remove('active'));
    els.closeRotationModalCancelBtn.addEventListener('click', () => els.rotationModal.classList.remove('active'));
    els.rotationForm.addEventListener('submit', onCreateRotationPattern);
    els.addRotationStepBtn.addEventListener('click', () => addRotationStep());
  }

  // Shift Swap Event Listeners
  if (els.openShiftSwapModalBtn) {
    els.openShiftSwapModalBtn.addEventListener('click', () => {
      els.shiftSwapForm.reset();
      els.shiftSwapModal.classList.add('active');
    });
    els.closeShiftSwapModalBtn.addEventListener('click', () => els.shiftSwapModal.classList.remove('active'));
    els.closeShiftSwapModalCancelBtn.addEventListener('click', () => els.shiftSwapModal.classList.remove('active'));
    els.shiftSwapForm.addEventListener('submit', onCreateShiftSwap);
  }
  els.closeAssignShiftModalBtn.addEventListener('click', () => els.assignShiftModal.classList.remove('active'));
  els.closeAssignShiftModalCancelBtn.addEventListener('click', () => els.assignShiftModal.classList.remove('active'));
  els.assignShiftForm.addEventListener('submit', onAssignShift);
  els.deviceSearch.addEventListener('input', renderDevices);
  els.employeeSearch.addEventListener('input', renderEmployees);
  if (els.startCameraButton) els.startCameraButton.addEventListener('click', startCameraAttendance);
  if (els.stopCameraButton) els.stopCameraButton.addEventListener('click', stopCameraAttendance);
  if (els.closeRegisterFaceModalBtn) {
    els.closeRegisterFaceModalBtn.addEventListener('click', () => {
      stopRegisterFaceCamera();
      if (els.registerFaceModal) els.registerFaceModal.classList.remove('active');
      document.body.classList.remove('modal-open');
    });
  }
  if (els.startRegisterFaceCamBtn) els.startRegisterFaceCamBtn.addEventListener('click', startRegisterFaceCamera);
  if (els.captureFaceBtn) els.captureFaceBtn.addEventListener('click', captureAndSaveFace);
  els.openNewEssModalBtn.addEventListener('click', async () => {
    els.essForm.reset();
    if (!state.employees || state.employees.length === 0) {
      await loadEmployees();
    }
    populateEssEmployees();
    toggleEssFormFields();
    els.essModal.classList.add('active');
  });
  els.closeEssModalBtn.addEventListener('click', () => els.essModal.classList.remove('active'));
  els.closeEssModalCancelBtn.addEventListener('click', () => els.essModal.classList.remove('active'));
  els.loadAttendanceBtn.addEventListener('click', pullAttendanceViaSDK);
  els.applyAttendanceBtn.addEventListener('click', loadAttendance);
  els.showAttendanceSummaryBtn.addEventListener('click', () => switchAttendanceSection('summary'));
  els.showAttendanceRawBtn.addEventListener('click', () => switchAttendanceSection('raw'));
  els.importEmployeesBtn.addEventListener('click', () => window.importEmployees?.());
  els.syncNowBtn.addEventListener('click', () => {
    showToast(window._lang === 'vi' ? 'Đang bắt đầu đồng bộ dữ liệu...' : 'Starting data sync...', 'info');
    syncNow();
  });
  els.processAttendanceBtn.addEventListener('click', () => {
    showToast(window._lang === 'vi' ? 'Đang xử lý chấm công...' : 'Processing attendance...', 'info');
    onProcessAttendance();
  });
  els.applyReportBtn.addEventListener('click', () => {
    showToast(window._lang === 'vi' ? 'Đang tải báo cáo...' : 'Loading report...', 'info');
    loadMonthlyReport();
  });
  els.exportExcelBtn.addEventListener('click', () => {
    showToast(window._lang === 'vi' ? 'Đang xuất báo cáo Excel...' : 'Exporting Excel report...', 'info');
    exportMonthlyReport();
  });
  els.sendMonthlyReportBtn.addEventListener('click', sendMonthlyReportsByEmail);
  els.refreshAuditBtn.addEventListener('click', loadAuditLogs);
  els.essForm.addEventListener('submit', onSubmitEssRequest);
  document.addEventListener('click', onTableAction);

  // Fingerprint Modal events. Some deployments do not include every optional
  // fingerprint control, so each binding must be independent. A missing
  // control must not prevent the employee toolbar bindings below from running.
  if (els.closeFingerprintModalBtn) {
    els.closeFingerprintModalBtn.addEventListener('click', () => {
      state.selectedFingerprintEmployeeId = null;
      els.fingerprintModal.classList.remove('active');
    });
  }
  if (els.fpTriggerEnrollBtn) els.fpTriggerEnrollBtn.addEventListener('click', triggerRemoteEnroll);
  if (els.fpReEnrollBtn) els.fpReEnrollBtn.addEventListener('click', () => reEnrollFingerprint());
  if (els.fpPushToDevicesBtn) els.fpPushToDevicesBtn.addEventListener('click', pushFingerprintsToAll);
  if (els.fpDeleteBtn) els.fpDeleteBtn.addEventListener('click', deleteFingerprint);
  if (els.fpManualConfirmLink) els.fpManualConfirmLink.addEventListener('click', (e) => {
    e.preventDefault();
    if (state.selectedFingerprintEmployeeId) {
      confirmFingerprintEnrollment(state.selectedFingerprintEmployeeId);
    }
  });

  // New sync modal events
  const pushAllToDeviceBtn = els.pushAllNowBtn;
  const closePushAllModalBtn = document.getElementById('closePushAllModalBtn');
  const cancelPushAllBtn = document.getElementById('cancelPushAllBtn');
  const confirmPushAllBtn = document.getElementById('confirmPushAllBtn');
  const pushAllToDeviceModal = document.getElementById('pushAllToDeviceModal');
  // The toolbar already contains the selected target device, so execute the
  // requested synchronization directly instead of opening a second selector.
  if (pushAllToDeviceBtn) pushAllToDeviceBtn.addEventListener('click', runPushAllToDeviceNow);
  if (closePushAllModalBtn) closePushAllModalBtn.addEventListener('click', () => pushAllToDeviceModal.classList.remove('active'));
  if (cancelPushAllBtn) cancelPushAllBtn.addEventListener('click', () => pushAllToDeviceModal.classList.remove('active'));
  if (confirmPushAllBtn) confirmPushAllBtn.addEventListener('click', confirmPushAllToDevice);

  const batchEnrollBtn = document.getElementById('batchEnrollBtn');
  const batchEnrollAllBtn = document.getElementById('batchEnrollAllBtn');
  const closeBatchEnrollModalBtn = document.getElementById('closeBatchEnrollModalBtn');
  const cancelBatchEnrollBtn = document.getElementById('cancelBatchEnrollBtn');
  const confirmBatchEnrollBtn = document.getElementById('confirmBatchEnrollBtn');
  const stopBatchEnrollBtn = document.getElementById('stopBatchEnrollBtn');
  const stopBatchEnrollModalBtn = document.getElementById('stopBatchEnrollModalBtn');
  const batchEnrollModal = document.getElementById('batchEnrollModal');
  const batchEnrollSelectAll = document.getElementById('batchEnrollSelectAll');
  if (batchEnrollBtn) batchEnrollBtn.addEventListener('click', openBatchEnrollWizard);
  if (batchEnrollAllBtn) batchEnrollAllBtn.addEventListener('click', runBatchEnrollForPendingEmployees);
  // Fallback: if elements weren't found at bind time (rare), use delegated listeners
  if (!batchEnrollBtn) {
    console.warn('batchEnrollBtn not found during bind, adding delegated listener');
    document.addEventListener('click', (e) => {
      const target = e.target || e.srcElement;
      if (target && target.id === 'batchEnrollBtn') {
        openBatchEnrollWizard();
      }
    });
  }
  if (!batchEnrollAllBtn) {
    console.warn('batchEnrollAllBtn not found during bind, adding delegated listener');
    document.addEventListener('click', (e) => {
      const target = e.target || e.srcElement;
      if (target && target.id === 'batchEnrollAllBtn') {
        runBatchEnrollForPendingEmployees();
      }
    });
  }
  if (closeBatchEnrollModalBtn) closeBatchEnrollModalBtn.addEventListener('click', () => batchEnrollModal.classList.remove('active'));
  if (cancelBatchEnrollBtn) cancelBatchEnrollBtn.addEventListener('click', () => batchEnrollModal.classList.remove('active'));
  if (confirmBatchEnrollBtn) confirmBatchEnrollBtn.addEventListener('click', confirmBatchEnroll);
  if (stopBatchEnrollBtn) stopBatchEnrollBtn.addEventListener('click', () => stopBatchEnroll('stopBatchEnrollDeviceSelect'));
  if (stopBatchEnrollModalBtn) stopBatchEnrollModalBtn.addEventListener('click', () => stopBatchEnroll('batchEnrollDeviceSelect'));
  if (batchEnrollSelectAll) batchEnrollSelectAll.addEventListener('change', (e) => {
    document.querySelectorAll('#batchEnrollEmployeeList input[type="checkbox"]').forEach(cb => cb.checked = e.target.checked);
  });

  const pullFromDeviceBtn = document.getElementById('pullFromDeviceBtn');
  const closePullFromDeviceModalBtn = document.getElementById('closePullFromDeviceModalBtn');
  const cancelPullFromDeviceBtn = document.getElementById('cancelPullFromDeviceBtn');
  const confirmPullFromDeviceBtn = document.getElementById('confirmPullFromDeviceBtn');
  const pullFromDeviceModal = document.getElementById('pullFromDeviceModal');
  if (pullFromDeviceBtn) pullFromDeviceBtn.addEventListener('click', openPullFromDeviceModal);
  if (els.pullFromDeviceNowBtn) els.pullFromDeviceNowBtn.addEventListener('click', runPullFromDeviceNow);
  if (closePullFromDeviceModalBtn) closePullFromDeviceModalBtn.addEventListener('click', () => pullFromDeviceModal.classList.remove('active'));
  if (cancelPullFromDeviceBtn) cancelPullFromDeviceBtn.addEventListener('click', () => pullFromDeviceModal.classList.remove('active'));
  if (confirmPullFromDeviceBtn) confirmPullFromDeviceBtn.addEventListener('click', confirmPullFromDevice);

  // Sync Single Employee Modal events
  if (els.closeSyncSingleEmployeeModalBtn) els.closeSyncSingleEmployeeModalBtn.addEventListener('click', () => els.syncSingleEmployeeModal.classList.remove('active'));
  if (els.cancelSyncSingleEmployeeBtn) els.cancelSyncSingleEmployeeBtn.addEventListener('click', () => els.syncSingleEmployeeModal.classList.remove('active'));
  if (els.confirmSyncSingleEmployeeBtn) els.confirmSyncSingleEmployeeBtn.addEventListener('click', confirmSyncSingleEmployee);

  // Click Outside to Close all Dropdowns
  document.addEventListener('click', () => {
    document.querySelectorAll('.row-dropdown-content').forEach(d => d.classList.remove('active'));
  });

  // Every modal can be dismissed consistently by its backdrop or Escape.
  document.querySelectorAll('.modal').forEach((modal) => {
    const backdrop = modal.querySelector('.modal-backdrop');
    if (backdrop) backdrop.addEventListener('click', () => closeModalElement(modal));
  });
  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape') return;
    const activeModals = document.querySelectorAll('.modal.active');
    if (activeModals.length > 0) closeModalElement(activeModals[activeModals.length - 1]);
  });

  // Conditionally Show Excel Import Button
  const employeeFileInput = document.getElementById('employeeFileInput');
  const importEmployeesBtn = document.getElementById('importEmployeesBtn');
  if (employeeFileInput && importEmployeesBtn) {
    employeeFileInput.addEventListener('change', () => {
    });
  }
}

function switchView(viewName) {
  if (!viewName) return;
  state.activeView = viewName;

  document.querySelectorAll('.view').forEach((view) => {
    view.classList.toggle('active', view.id === `${viewName}View`);
  });

  document.querySelectorAll('.nav-item').forEach((btn) => {
    btn.classList.toggle('active', btn.dataset.view === viewName);
  });

  const titles = {
    dashboard: { vi: 'Tổng quan', en: 'Dashboard' },
    devices: { vi: 'Thiết bị', en: 'Devices' },
    employees: { vi: 'Nhân viên', en: 'Employees' },
    shifts: { vi: 'Ca & Gán ca', en: 'Shifts' },
    ess: { vi: 'Đơn từ & ESS', en: 'ESS Requests' },
    attendance: { vi: 'Chấm công', en: 'Attendance' },
    reports: { vi: 'Báo cáo tháng', en: 'Monthly Reports' },
    audit: { vi: 'Nhật ký hệ thống', en: 'Audit Logs' },
    sync: { vi: 'Đồng bộ', en: 'Sync' },
  }
}

function showLogin() {
  els.loginView.classList.remove('hidden');
  els.mainApp.classList.add('hidden');
}

function showApp() {
  els.loginView.classList.add('hidden');
  els.mainApp.classList.remove('hidden');
  els.currentUser.textContent = state.user?.username || 'Admin';
}

async function onLogin(event) {
  if (event) event.preventDefault();
  const username = (els.username ? els.username.value : 'admin').trim();
  const password = els.password ? els.password.value : 'admin123';
  try {
    const response = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || (window._lang === 'vi' ? 'Đăng nhập thất bại' : 'Login failed'));
    state.token = data.token;
    state.user = { username };
    localStorage.setItem('attendance-token', state.token);
    localStorage.setItem('attendance-user', JSON.stringify(state.user));
    showApp();
    loadAll();
    connectSSE();
    startAttendanceAutoRefresh();
    if (els.loginMessage) {
      els.loginMessage.textContent = window._lang === 'vi' ? 'Đăng nhập thành công' : 'Login successful';
      els.loginMessage.classList.remove('error');
    }
  } catch (error) {
    console.error('Login error:', error);
    if (els.loginMessage) {
      els.loginMessage.textContent = error.message;
      els.loginMessage.classList.add('error');
    } else {
      showToast(error.message, 'error');
    }
  }
}
window.onLogin = onLogin;

function logout() {
  if (eventSource) {
    try { eventSource.close(); } catch(e) {}
    eventSource = null;
  }
  state.token = '';
  state.user = null;
  localStorage.removeItem('attendance-token');
  localStorage.removeItem('attendance-user');
  showLogin();
}

async function loadAll() {
  const [deviceResult, employeeResult, attendanceResult, historyResult, statsResult, shiftResult, auditResult] = await Promise.allSettled([
    loadDevices(),
    loadEmployees(),
    loadAttendance(),
    loadSyncHistory(),
    loadDashboardStats(),
    loadShifts(),
    loadAuditLogs()
  ]);
  renderDashboard();
  renderDevices();
  renderEmployees();
  renderAttendance();
  renderSyncHistory();
  renderShifts();
  renderAuditLogs();
  if (deviceResult.status === 'rejected') console.warn(deviceResult.reason);
  if (employeeResult.status === 'rejected') console.warn(employeeResult.reason);
  if (attendanceResult.status === 'rejected') console.warn(attendanceResult.reason);
  if (historyResult.status === 'rejected') console.warn(historyResult.reason);
  if (statsResult.status === 'rejected') console.warn(statsResult.reason);
  if (shiftResult.status === 'rejected') console.warn(shiftResult.reason);
  if (auditResult.status === 'rejected') console.warn(auditResult.reason);
}

window.importEmployees = async function importEmployees() {
  const fileInput = document.getElementById('employeeFileInput');
  const file = fileInput?.files?.[0];
  if (!file) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn file Excel (.xlsx).' : 'Please select an Excel file (.xlsx).', 'error');
    return;
  }

  try {
    const formData = new FormData();
    formData.append('file', file);
    const response = await api('/api/v1/employees/import', { method: 'POST', body: formData });
    const result = await response.json();

    if (result.failed > 0) {
      showToast(window._lang === 'vi'
        ? `Import xong: ${result.imported} thành công, ${result.failed} thất bại.`
        : `Import complete: ${result.imported} succeeded, ${result.failed} failed.`, 'error');
    } else {
      showToast(window._lang === 'vi'
        ? `Import thành công: ${result.imported} nhân viên.`
        : `Import successful: ${result.imported} employees.`, 'success');
    }

    if (fileInput) fileInput.value = '';
    await loadEmployees();
    renderEmployees();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function loadDevices() {
  try {
    const response = await api('/api/v1/devices');
    const data = await response.json();
    state.devices = Array.isArray(data) ? data : fallbackDevices;
  } catch {
    state.devices = fallbackDevices;
  }
  updateEmployeeEnrollDeviceOptions();
  populateStopBatchDeviceSelect();
  populateToolbarBackupSelects();
  populateSyncDeviceSelect();
}

function updateEmployeeEnrollDeviceOptions() {
  if (!els.employeeEnrollDeviceId) return;
  const admsDevices = state.devices.filter(Boolean);
  if (admsDevices.length === 0) {
    els.employeeEnrollDeviceId.innerHTML = '<option value="">-- Không có thiết bị ADMS --</option>';
    return;
  }
  els.employeeEnrollDeviceId.innerHTML = admsDevices.map(d => `<option value="${d.id}">${d.name} (${d.adms_enabled ? 'ADMS' : 'SDK'})</option>`).join('');
}

function populateSyncDeviceSelect() {
  const select = els.syncDeviceSelect;
  if (!select) return;
  const admsDevices = (state.devices || []).filter(Boolean);
  if (admsDevices.length === 0) {
    select.innerHTML = '<option value="">-- Không có thiết bị ADMS --</option>';
    return;
  }
  select.innerHTML = ['<option value="">-- Chọn thiết bị để đồng bộ --</option>'].concat(admsDevices.map(d => `<option value="${d.id}">${d.name} (${d.adms_enabled ? 'ADMS' : 'SDK'}) ${d.status === 'online' ? '✅' : '⚠'}</option>`)).join('');
}

async function runPushAllToDeviceNow() {
  const deviceId = els.syncDeviceSelect?.value;
  if (!deviceId) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị để đẩy nhân viên!' : 'Please select a device to push employees to!', 'warning');
    return;
  }
  const device = state.devices.find(d => d.id === deviceId);
  const deviceName = device?.name || deviceId;
  try {
    // Validate device is ADMS-enabled and currently online
    if (!device) throw new Error(window._lang === 'vi' ? 'Thiết bị không tồn tại' : 'Device not found');
    if (device.adms_enabled && device.status !== 'online') throw new Error(window._lang === 'vi' ? 'Thiết bị ADMS hiện không online, không thể đẩy dữ liệu' : 'ADMS device is not online; cannot push data');
    const btn = els.pushAllNowBtn;
    const orig = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '⏳ Đang xử lý...';
    const response = await api(`/api/v1/devices/${deviceId}/sync-employees`, { method: 'POST' });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || data.message || 'Failed');
    const msg = window._lang === 'vi' ? `✅ Đã đưa ${data.record_count || '?'} nhân viên vào hàng đợi máy "${deviceName}"!` : `✅ Queued ${data.record_count || '?'} employees to device "${deviceName}"!`;
    els.employeeMessage.className = 'message success';
    els.employeeMessage.textContent = msg;
    els.employeeMessage.style.display = 'block';
    showToast(msg, 'success');
    await loadAll();
  } catch (err) {
    const errMsg = err.message || 'Failed to push employees';
    els.employeeMessage.className = 'message error';
    els.employeeMessage.textContent = errMsg;
    els.employeeMessage.style.display = 'block';
    showToast(errMsg, 'error');
  } finally {
    els.pushAllNowBtn.disabled = false;
    els.pushAllNowBtn.innerHTML = '📤 Đẩy tất cả xuống máy';
  }
}

async function runPullFromDeviceNow() {
  const deviceId = els.syncDeviceSelect?.value;
  if (!deviceId) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị nguồn để kéo!' : 'Please select a source device to pull from!', 'warning');
    return;
  }
  const device = state.devices.find(d => d.id === deviceId);
  const deviceName = device?.name || deviceId;
  const statusEl = els.employeeMessage;
  try {
    const btn = els.pullFromDeviceNowBtn;
    const orig = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '⏳ Đang kéo dữ liệu...';
    statusEl.className = 'message';
    statusEl.textContent = window._lang === 'vi' ? `⏳ Đang kết nối và kéo nhân viên từ "${deviceName}". Vui lòng chờ...` : `⏳ Connecting and pulling employees from "${deviceName}". Please wait...`;
    statusEl.style.display = 'block';

    const response = await api(`/api/v1/devices/${deviceId}/pull-employees`, { method: 'POST' });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to pull employees');

    const imported = Number(data?.imported || 0);
    const existing = Number(data?.existing || 0);
    const errors = Array.isArray(data?.errors) ? data.errors.filter(Boolean) : [];
    statusEl.className = errors.length ? 'message warning' : 'message success';
    statusEl.textContent = window._lang === 'vi'
      ? `${errors.length ? '⚠️' : '✅'} Kéo từ "${deviceName}" hoàn tất. Mới: ${imported}; đã có: ${existing}.${errors.length ? ` Lỗi: ${errors.join(' | ')}` : ''}`
      : `${errors.length ? '⚠️' : '✅'} Pull from "${deviceName}" completed. New: ${imported}; existing: ${existing}.${errors.length ? ` Errors: ${errors.join(' | ')}` : ''}`;
    showToast(window._lang === 'vi'
      ? `${errors.length ? 'Kéo dữ liệu hoàn tất nhưng có lỗi.' : 'Kéo nhân viên thành công.'} Mới: ${imported}, đã có: ${existing}.`
      : `${errors.length ? 'Pull completed with errors.' : 'Employees pulled successfully.'} New: ${imported}, existing: ${existing}.`, errors.length ? 'warning' : 'success');

    await loadEmployees();
    renderEmployees();
  } catch (err) {
    statusEl.className = 'message error';
    statusEl.textContent = window._lang === 'vi'
      ? `❌ Kéo nhân viên từ "${deviceName}" thất bại: ${err?.message || 'Không xác định được lỗi.'}`
      : `❌ Failed to pull employees from "${deviceName}": ${err?.message || 'Unknown error.'}`;
    statusEl.style.display = 'block';
    showToast(statusEl.textContent, 'error');
  } finally {
    els.pullFromDeviceNowBtn.disabled = false;
    els.pullFromDeviceNowBtn.innerHTML = '📥 Kéo NV từ máy';
  }
}

function populateToolbarBackupSelects() {
  if (!els.toolbarBackupSourceSelect || !els.toolbarBackupTargetSelect) return;
  // Fill source select
  els.toolbarBackupSourceSelect.innerHTML = `<option value="">-- Chọn nguồn sao chép --</option>` + state.devices.map(d => `<option value="${d.id}">${d.name} (${d.ip_address || 'N/A'})</option>`).join('');
  // Fill targets (same list, allow multi-select)
  els.toolbarBackupTargetSelect.innerHTML = state.devices.map(d => `<option value="${d.id}">${d.name} (${d.ip_address || 'N/A'})</option>`).join('');
}

async function runToolbarBackup() {
  const srcId = els.toolbarBackupSourceSelect?.value;
  if (!srcId) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị nguồn để sao chép!' : 'Please select a source device to backup from!', 'warning');
    return;
  }
  const selected = Array.from(els.toolbarBackupTargetSelect.selectedOptions || []).map(o => o.value).filter(v => v && v !== srcId);
  if (selected.length === 0) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn ít nhất một thiết bị đích khác nguồn!' : 'Please select at least one target device different from source!', 'warning');
    return;
  }

  const orig = els.toolbarBackupBtn.innerHTML;
  els.toolbarBackupBtn.disabled = true;
  els.toolbarBackupBtn.innerHTML = '⏳ Đang sao chép...';
  try {
    const response = await api(`/api/v1/devices/${srcId}/backup`, { method: 'POST', body: JSON.stringify({ target_device_ids: selected }) });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || data.message || 'Failed to backup');
    showToast(window._lang === 'vi' ? 'Sao chép thành công!' : 'Backup completed!', 'success');
    await loadAll();
  } catch (err) {
    showToast(err.message || 'Backup failed', 'error');
  } finally {
    els.toolbarBackupBtn.disabled = false;
    els.toolbarBackupBtn.innerHTML = orig;
  }
}

function toggleEmployeeEnrollFields() {
  if (!els.employeeEnrollSection || !els.employeeEnrollFingerprint) return;
  els.employeeEnrollSection.style.display = els.employeeEnrollFingerprint.checked ? 'block' : 'none';
  if (els.employeeEnrollFingerprint.checked && els.employeeEnrollDeviceUserId && els.employeeCode) {
    els.employeeEnrollDeviceUserId.value = els.employeeEnrollDeviceUserId.value || els.employeeCode.value;
  }
}

function resetEmployeeForm() {
  state.editingEmployeeId = '';
  els.employeeForm.reset();
  els.employeeId.value = '';
  if (els.employeeEnrollFingerprint) {
    els.employeeEnrollFingerprint.checked = false;
  }
  if (els.employeeEnrollSection) {
    els.employeeEnrollSection.style.display = 'none';
  }
  if (els.employeeEnrollDeviceUserId) {
    els.employeeEnrollDeviceUserId.value = '';
  }
}

async function loadEmployees() {
  try {
    const response = await api('/api/v1/employees');
    const data = await response.json();
    state.employees = Array.isArray(data) ? data : fallbackEmployees;
  } catch {
    state.employees = fallbackEmployees;
  }
  populateEssEmployees();
}

async function loadDashboardStats() {
  try {
    const response = await api('/api/v1/dashboard/stats');
    const data = await response.json();
    state.dashboardStats = data || {};
  } catch {
    state.dashboardStats = {};
  }
  renderDashboard();
}

async function loadAttendance() {
  try {
    const params = new URLSearchParams();
    // Dùng local time (không hardcode Z=UTC) để filter đúng múi giờ Việt Nam UTC+7
    if (els.attendanceFrom.value) params.set('from', new Date(els.attendanceFrom.value + 'T00:00:00').toISOString());
    if (els.attendanceTo.value) params.set('to', new Date(els.attendanceTo.value + 'T23:59:59').toISOString());
    if (els.attendanceCode.value) params.set('employee_code', els.attendanceCode.value);
    const response = await api(`/api/v1/attendance-logs${params.toString() ? `?${params}` : ''}`);
    const data = await response.json();
    state.attendance = Array.isArray(data) ? data : fallbackAttendance;
    const latest = state.attendance[0];
    if (latest) {
      // Keep the latest-scan card working even when an intermediary proxy
      // buffers SSE events; the attendance table remains the source of truth.
      showLastScan({
        latest_employee_code: latest.employee_code,
        latest_employee_name: latest.employee_name,
        latest_check_time: latest.check_time,
        latest_check_type: latest.check_type,
        latest_verify_mode: latest.verify_mode,
        device_name: latest.device_name
      });
    }
  } catch {
    state.attendance = fallbackAttendance;
  }
  try {
    const params = new URLSearchParams();
    const fromStr = els.attendanceFrom.value || getLocalDateStr(new Date(Date.now() - 7*24*60*60*1000));
    const toStr = els.attendanceTo.value || getLocalDateStr(new Date());
    params.set('from', fromStr);
    params.set('to', toStr);
    if (els.attendanceCode.value) {
      const emp = state.employees.find(e => e.employee_code === els.attendanceCode.value || e.id === els.attendanceCode.value);
      if (emp) {
        params.set('employee_id', emp.id);
      } else {
        params.set('employee_id', els.attendanceCode.value);
      }
    }
    const response = await api(`/api/v1/daily-attendance/report?${params.toString()}`);
    const data = await response.json();
    state.attendanceSummary = Array.isArray(data) ? data : [];
  } catch (err) {
    console.error('Failed to load daily attendance report:', err);
    state.attendanceSummary = [];
  }
  renderAttendance();
}

async function loadSyncHistory() {
  try {
    const response = await api('/api/v1/sync-history');
    const data = await response.json();
    state.syncHistory = Array.isArray(data) ? data : fallbackHistory;
  } catch {
    state.syncHistory = fallbackHistory;
  }
}

async function onSaveDevice(event) {
  event.preventDefault();
  const payload = {
    name: els.deviceName.value,
    device_type: els.deviceType.value,
    ip_address: els.deviceIp.value,
    port: Number(els.devicePort.value || 4370),
    serial_number: els.deviceSerial.value,
    serial_number_adms: els.deviceSerialADMS.value,
    adms_enabled: els.deviceADMSEnabled.checked,
    firmware_version: els.deviceFirmware.value,
    mac_address: els.deviceMac.value,
    location: els.deviceLocation.value
  };
  try {
    let response;
    if (state.editingDeviceId) {
      response = await api(`/api/v1/devices/${state.editingDeviceId}`, { method: 'PUT', body: JSON.stringify(payload) });
    } else {
      response = await api('/api/v1/devices', { method: 'POST', body: JSON.stringify(payload) });
    }
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || (window._lang === 'vi' ? 'Không thể lưu thiết bị' : 'Failed to save device'));
    showToast(t('toastSaveDevice'), 'success');
    resetDeviceForm();
    els.deviceModal.classList.remove('active'); // Đóng modal
    loadAll();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function onSaveEmployee(event) {
  console.log('[debug] onSaveEmployee clicked');
  event.preventDefault();
  try {
    const payload = {
      employee_code: els.employeeCode.value,
      full_name: els.employeeName.value,
      department_id: els.employeeDepartment.value,
      card_no: els.employeeCard.value,
      job_title: els.employeeJobTitle.value,
      email: els.employeeEmail.value,
      phone: els.employeePhone.value,
      zalo_user_id: els.employeeZaloUserId ? els.employeeZaloUserId.value.trim() : '',
      gender: els.employeeGender.value,
      dob: els.employeeDob.value || null,
      join_date: els.employeeJoinDate.value || null,
      avatar_url: els.employeeAvatar.value
    };
    const isNew = !state.editingEmployeeId;
    if (isNew && els.employeeEnrollFingerprint && els.employeeEnrollFingerprint.checked) {
      payload.enroll_fingerprint = true;
      payload.device_id = els.employeeEnrollDeviceId ? els.employeeEnrollDeviceId.value : '';
      payload.device_user_id = els.employeeEnrollDeviceUserId ? els.employeeEnrollDeviceUserId.value.trim() : '';
    }
    let response;
    if (state.editingEmployeeId) {
      response = await api(`/api/v1/employees/${state.editingEmployeeId}`, { method: 'PUT', body: JSON.stringify({ ...payload, status: els.employeeStatus.value }) });
    } else {
      response = await api('/api/v1/employees', { method: 'POST', body: JSON.stringify(payload) });
    }
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || (window._lang === 'vi' ? 'Không thể lưu nhân viên' : 'Failed to save employee'));
    showToast(t('toastSaveEmployee'), 'success');
    resetEmployeeForm();
    els.employeeModal.classList.remove('active');
    await loadEmployees();
    renderEmployees();

    if (isNew && data?.id && payload.enroll_fingerprint && payload.device_id) {
      openSyncSingleEmployeeModal(data.id, payload.employee_code, payload.full_name);
    }
  } catch (error) {
    console.error('onSaveEmployee failed:', error);
    showToast(error?.message || (window._lang === 'vi' ? 'Không thể lưu nhân viên' : 'Failed to save employee'), 'error');
  }
}

async function syncNow() {
  if (!state.devices.length) {
    showToast(t('toastNoDeviceSync'), 'error');
    return;
  }

  const btn = els.syncNowBtn;
  if (btn) {
    btn.disabled = true;
    btn.textContent = window._lang === 'vi' ? '⏳ Đang đồng bộ...' : '⏳ Syncing...';
  }

  const devices = state.devices;
  let successCount = 0;
  let failCount = 0;

  // Theo dõi trạng thái từng thiết bị để hiện tiến độ real-time
  const deviceStatus = devices.map(d => ({ id: d.id, name: d.name, status: 'pending' }));

  function renderSyncProgress() {
    const isVI = window._lang === 'vi';
    const parts = deviceStatus.map(ds => {
      if (ds.status === 'pending') return `⏳ ${ds.name}`;
      if (ds.status === 'success') return `✅ ${ds.name}`;
      return `❌ ${ds.name}`;
    });
    els.syncMessage.className = 'message info';
    els.syncMessage.textContent = (isVI ? 'Đang đồng bộ: ' : 'Syncing: ') + parts.join('  |  ');
  }

  renderSyncProgress();
  showToast(`${t('toastSyncStart')} ${devices.length} ${window._lang === 'vi' ? 'thiết bị' : 'devices'}...`, 'info');

  try {
    // Đồng bộ song song - mỗi thiết bị độc lập, cập nhật UI ngay khi có kết quả
    const syncPromises = devices.map(async (device, index) => {
      try {
        // COM SDK cần thời gian Connect_Net và nạp ATTLOG; timeout 30 giây
        // phải khớp với giới hạn của API phía server.
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 30000);
        let response;
        try {
          response = await api(`/api/v1/devices/${device.id}/sync-attendance`, {
            method: 'POST',
            body: JSON.stringify({}),
            signal: controller.signal
          });
        } finally {
          clearTimeout(timeoutId);
        }

        const data = await response.json();
        if (!response.ok) throw new Error(data.error || 'Sync failed');

        successCount++;
        deviceStatus[index].status = 'success';
        renderSyncProgress();

        // Cập nhật bảng lịch sử đồng bộ ngay sau khi thiết bị này thành công
        await loadSyncHistory();
        renderSyncHistory();

      } catch (error) {
        failCount++;
        deviceStatus[index].status = 'error';
        renderSyncProgress();

        const isTimeout = error.name === 'AbortError';
        const errMsg = isTimeout
          ? (window._lang === 'vi' ? 'Hết thời gian chờ – thiết bị offline?' : 'Timeout – device offline?')
          : error.message;
        showToast(window._lang === 'vi'
          ? `❌ ${device.name}: ${errMsg}`
          : `❌ ${device.name}: ${errMsg}`, 'error');
      }
    });

    // Chờ tất cả thiết bị hoàn tất (allSettled = không throw dù có lỗi)
    await Promise.allSettled(syncPromises);

    // Tải lại toàn bộ dữ liệu Dashboard + Chấm công + Báo cáo
    await loadAll();

    // Hiện kết quả tổng hợp
    if (successCount > 0 && failCount === 0) {
      els.syncMessage.className = 'message success';
      els.syncMessage.textContent = window._lang === 'vi'
        ? `✅ Đồng bộ hoàn tất: ${successCount}/${devices.length} thiết bị thành công. Dữ liệu đã được cập nhật.`
        : `✅ Sync complete: ${successCount}/${devices.length} devices succeeded. Data refreshed.`;
      showToast(window._lang === 'vi'
        ? `Đồng bộ thành công ${successCount} thiết bị!`
        : `Successfully synced ${successCount} devices!`, 'success');
    } else if (successCount > 0) {
      els.syncMessage.className = 'message warning';
      els.syncMessage.textContent = window._lang === 'vi'
        ? `⚠️ Hoàn tất: ${successCount} thành công, ${failCount} thất bại. Dữ liệu đã cập nhật từ thiết bị thành công.`
        : `⚠️ Done: ${successCount} succeeded, ${failCount} failed. Data refreshed from successful devices.`;
      showToast(window._lang === 'vi'
        ? `Đồng bộ hoàn tất: ${successCount} thành công, ${failCount} thất bại`
        : `Sync done: ${successCount} ok, ${failCount} failed`, 'info');
    } else {
      els.syncMessage.className = 'message error';
      els.syncMessage.textContent = window._lang === 'vi'
        ? `❌ Tất cả ${failCount} thiết bị thất bại. Kiểm tra kết nối mạng và trạng thái máy chấm công.`
        : `❌ All ${failCount} devices failed. Check network and device status.`;
    }

  } catch (error) {
    els.syncMessage.className = 'message error';
    els.syncMessage.textContent = error.message;
    showToast(error.message, 'error');
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = window._lang === 'vi' ? '▶ Đồng bộ ngay' : '▶ Sync Now';
    }
  }
}

function renderDashboard() {
  if (els.deviceCount) els.deviceCount.textContent = state.devices.length;
  if (els.employeeCount) els.employeeCount.textContent = state.employees.length;
  if (els.attendanceCount) els.attendanceCount.textContent = state.attendance.length;
  if (els.syncCount) els.syncCount.textContent = state.syncHistory.length;

  if (els.recentDevices) {
    const listHtml = state.devices.slice(0, 5).map(d => {
      const statusClass = (d.status || 'offline') === 'online' ? 'online' : 'offline';
      const statusText = (d.status || 'offline') === 'online' ? 'Online' : 'Offline';
      return `
        <div class="list-item" style="display:flex; justify-content:space-between; align-items:center; padding:10px; border-bottom:1px solid var(--border);">
          <div>
            <strong>${d.name}</strong>
            <div class="muted" style="font-size:0.8rem;">IP: ${d.ip_address || '-'}</div>
          </div>
          <span class="badge ${statusClass}">${statusText}</span>
        </div>
      `;
    }).join('');
    els.recentDevices.innerHTML = listHtml || `<div class="muted" style="padding:10px;">${window._lang === 'vi' ? 'Không có thiết bị' : 'No devices'}</div>`;
  }

  if (els.recentSync) {
    const listHtml = state.syncHistory.slice(0, 5).map(h => {
      const device = state.devices.find(d => d.id === h.device_id);
      const deviceName = device ? device.name : (h.device_id || 'Device');
      const statusClass = h.status === 'success' ? 'success' : 'error';
      const statusText = h.status === 'success' ? (window._lang === 'vi' ? 'Thành công' : 'Success') : (window._lang === 'vi' ? 'Lỗi' : 'Error');
      return `
        <div class="list-item" style="display:flex; justify-content:space-between; align-items:center; padding:10px; border-bottom:1px solid var(--border);">
          <div>
            <strong>${deviceName}</strong>
            <div class="muted" style="font-size:0.8rem;">${h.sync_type === 'attendance' ? (window._lang === 'vi' ? 'Đồng bộ log công' : 'Sync logs') : (window._lang === 'vi' ? 'Đồng bộ nhân viên' : 'Sync employees')}</div>
          </div>
          <span class="badge ${statusClass === 'success' ? 'online' : 'offline'}">${statusText}</span>
        </div>
      `;
    }).join('');
    els.recentSync.innerHTML = listHtml || `<div class="muted" style="padding:10px;">${window._lang === 'vi' ? 'Không có đồng bộ gần đây' : 'No recent sync'}</div>`;
  }

  updateCharts();
}

function renderDevices() {
  const searchTerm = els.deviceSearch.value.trim().toLowerCase();
  const filtered = state.devices.filter(d => 
    (d.name || '').toLowerCase().includes(searchTerm) ||
    (d.ip_address || '').toLowerCase().includes(searchTerm) ||
    (d.device_type || '').toLowerCase().includes(searchTerm) ||
    (d.serial_number || '').toLowerCase().includes(searchTerm)
  );

  els.deviceTableBody.innerHTML = filtered.map(d => {
    const statusClass = (d.status || 'offline') === 'online' ? 'online' : 'offline';
    const statusText = (d.status || 'offline') === 'online' ? 'Online' : 'Offline';
    const lastActiveText = d.last_heartbeat_at ? new Date(d.last_heartbeat_at).toLocaleString(window._lang === 'vi' ? 'vi-VN' : 'en-US') : '-';
    
    let typeDesc = d.device_type ? d.device_type.toUpperCase() : 'ZKTeco';
    if (d.adms_enabled) {
      typeDesc += ' (ADMS Push)';
    }

    return `
      <tr>
        <td>
          <strong>${d.name}</strong><br/>
          <span class="muted" style="font-size:0.8rem;">SN: ${d.serial_number || '-'}</span>
        </td>
        <td>
          <strong>${typeDesc}</strong><br/>
          <span class="muted" style="font-size:0.8rem;">FW: ${d.firmware_version || '-'}</span>
        </td>
        <td>
          <code>${d.ip_address}:${d.port}</code><br/>
          <span class="muted" style="font-size:0.8rem;">MAC: ${d.mac_address || '-'}</span>
        </td>
        <td><span class="muted" style="font-size:0.9rem;">${lastActiveText}</span></td>
        <td><span class="badge ${statusClass}">${statusText}</span></td>
        <td>
          <div class="table-actions">
            <button class="secondary-btn" data-action="test-device" data-id="${d.id}">🔌 Test</button>
            <button class="secondary-btn" data-action="reboot-device" data-id="${d.id}">🔁 Reboot</button>
            <!-- Removed Pull/Sync/Delete Log/Reset actions per UX request -->
            <!-- per-row backup button removed (use toolbar backup controls) -->
            <button class="secondary-btn" data-action="edit-device" data-id="${d.id}">${t('btnEdit')}</button>
            <button class="secondary-btn" data-action="delete-device" data-id="${d.id}" style="color: var(--danger);">${t('btnDelete')}</button>
          </div>
        </td>
      </tr>
    `;
  }).join('');
}

function resetDeviceForm() {
  state.editingDeviceId = '';
  els.deviceForm.reset();
  els.deviceId.value = '';
  els.deviceADMSEnabled.checked = false;
}

function resetEmployeeForm() {
  state.editingEmployeeId = '';
  els.employeeForm.reset();
  els.employeeId.value = '';
}

async function onSaveDevice(event) {
  event.preventDefault();
  // Fallback: Nếu không có serial_number_adms, dùng serial_number
  const serialAdms = els.deviceSerialADMS.value || els.deviceSerial.value;
  const serialNum = els.deviceSerial.value || els.deviceSerialADMS.value;
  
  const payload = {
    name: els.deviceName.value,
    device_type: els.deviceType.value,
    ip_address: els.deviceIp.value,
    port: Number(els.devicePort.value),
    serial_number: serialNum,
    serial_number_adms: serialAdms,
    adms_enabled: els.deviceADMSEnabled.checked,
    firmware_version: els.deviceFirmware.value,
    mac_address: els.deviceMac.value,
    location: els.deviceLocation.value
  };
  try {
    let response;
    if (state.editingDeviceId) {
      response = await api(`/api/v1/devices/${state.editingDeviceId}`, { method: 'PUT', body: JSON.stringify(payload) });
    } else {
      response = await api('/api/v1/devices', { method: 'POST', body: JSON.stringify(payload) });
    }
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || (window._lang === 'vi' ? 'Không thể lưu thiết bị' : 'Failed to save device'));
    showToast(t('toastSaveDevice'), 'success');
    resetDeviceForm();
    els.deviceModal.classList.remove('active');
    loadAll();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function editDevice(id) {
  const d = state.devices.find(dev => dev.id === id);
  if (!d) return;
  state.editingDeviceId = id;
  els.deviceModalTitle.textContent = window._lang === 'vi' ? 'Sửa thiết bị' : 'Edit Device';
  
  els.deviceId.value = d.id;
  els.deviceName.value = d.name;
  els.deviceType.value = d.device_type;
  els.deviceIp.value = d.ip_address;
  els.devicePort.value = d.port;
  els.deviceSerial.value = d.serial_number || '';
  els.deviceSerialADMS.value = d.serial_number_adms || d.serial_number || '';
  els.deviceADMSEnabled.checked = d.adms_enabled;
  els.deviceFirmware.value = d.firmware_version || '';
  els.deviceMac.value = d.mac_address || '';
  els.deviceLocation.value = d.location || '';
  
  els.deviceModal.classList.add('active');
}

async function deleteDevice(id) {
  try {
    const response = await api(`/api/v1/devices/${id}`, { method: 'DELETE' });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || 'Failed to delete device');
    showToast(window._lang === 'vi' ? 'Đã xoá thiết bị thành công!' : 'Device deleted successfully!', 'success');
    loadAll();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function openBackupDeviceModal(id) {
  const srcDev = state.devices.find(d => d.id === id);
  if (!srcDev) return;

  els.backupSourceDeviceName.textContent = srcDev.name;
  els.backupSourceDeviceId.value = srcDev.id;

  const otherDevices = state.devices.filter(d => d.id !== id);
  if (otherDevices.length === 0) {
    els.backupTargetDevicesList.innerHTML = `<p style="color: var(--muted); font-size: 0.9rem; text-align: center; margin: 15px 0;">${window._lang === 'vi' ? 'Không có thiết bị khác để sao chép' : 'No other devices available'}</p>`;
    els.backupSubmitBtn.disabled = true;
  } else {
    els.backupSubmitBtn.disabled = false;
    els.backupTargetDevicesList.innerHTML = otherDevices.map(d => {
      const typeLabel = d.adms_enabled ? 'ADMS' : 'PULL';
      return `
        <label style="display: flex; align-items: center; gap: 10px; cursor: pointer; padding: 6px; border-radius: 4px; transition: background 0.2s;" class="hover-bg">
          <input type="checkbox" name="backupTargetDevice" value="${d.id}" style="width: 18px; height: 18px; cursor: pointer;" />
          <div style="display: flex; flex-direction: column;">
            <strong style="font-size: 0.95rem; color: var(--text);">${d.name}</strong>
            <span style="font-size: 0.8rem; color: var(--muted);">${d.ip_address} (${typeLabel})</span>
          </div>
        </label>
      `;
    }).join('');
  }

  els.backupDeviceModal.classList.add('active');
}

async function onSubmitBackup(event) {
  // Nút sao chép là button thường, không phải submit event. Ở phiên bản cũ
  // `event` là undefined khi bấm nút nên thao tác dừng ngay sau toast.
  event?.preventDefault();
  const srcId = els.backupSourceDeviceId.value;
  const checkboxes = els.backupTargetDevicesList.querySelectorAll('input[name="backupTargetDevice"]:checked');
  const targetIds = Array.from(checkboxes).map(cb => cb.value);

  if (targetIds.length === 0) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn ít nhất một thiết bị nhận dữ liệu!' : 'Please select at least one target device!', 'warning');
    return;
  }

  const originalText = els.backupSubmitBtn.innerHTML;
  els.backupSubmitBtn.disabled = true;
  els.backupSubmitBtn.innerHTML = `<span>⏳ Đang sao chép...</span>`;

  try {
    const response = await api(`/api/v1/devices/${srcId}/backup`, {
      method: 'POST',
      body: JSON.stringify({ target_device_ids: targetIds })
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'Failed to backup device');
    showToast(window._lang === 'vi' ? 'Sao chép dữ liệu giữa các máy thành công!' : 'Device backup completed successfully!', 'success');
    els.backupDeviceModal.classList.remove('active');
    loadAll();
  } catch (error) {
    showToast(error.message, 'error');
  } finally {
    els.backupSubmitBtn.disabled = false;
    els.backupSubmitBtn.innerHTML = originalText;
  }
}

async function editEmployee(id) {
  const emp = state.employees.find(e => e.id === id);
  if (!emp) return;
  state.editingEmployeeId = id;
  els.employeeModalTitle.textContent = window._lang === 'vi' ? 'Sửa nhân viên' : 'Edit Employee';
  
  els.employeeId.value = emp.id;
  els.employeeCode.value = emp.employee_code;
  els.employeeName.value = emp.full_name;
  els.employeeJobTitle.value = emp.job_title || '';
  els.employeeDepartment.value = emp.department_id || '';
  els.employeeCard.value = emp.card_no || '';
  els.employeeEmail.value = emp.email || '';
  els.employeePhone.value = emp.phone || '';
  if (els.employeeZaloUserId) els.employeeZaloUserId.value = emp.zalo_user_id || '';
  els.employeeGender.value = emp.gender || 'male';
  els.employeeDob.value = emp.dob ? emp.dob.substring(0, 10) : '';
  els.employeeJoinDate.value = emp.join_date ? emp.join_date.substring(0, 10) : '';
  els.employeeAvatar.value = emp.avatar_url || '';
  els.employeeStatus.value = emp.status || 'active';
  
  els.employeeModal.classList.add('active');
}

async function deleteEmployee(id) {
  console.log('[debug] deleteEmployee called', id);
  try {
    const response = await api(`/api/v1/employees/${id}`, { method: 'DELETE' });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || (window._lang === 'vi' ? 'Không thể xoá nhân viên' : 'Failed to delete employee'));
    showToast(t('toastDeleteEmployeeSuccess'), 'success');
    await loadAll();
  } catch (error) {
    console.error('deleteEmployee failed:', error);
    showToast(error?.message || (window._lang === 'vi' ? 'Không thể xoá nhân viên' : 'Failed to delete employee'), 'error');
  }
}

function closeModalElement(modal) {
  if (!modal) return;
  modal.classList.remove('active');
  modal.style.removeProperty('display');
  if (!document.querySelector('.modal.active')) {
    document.body.classList.remove('modal-open');
  }
}

function confirmDeleteAllEmployees() {
  const count = state.employees.length;
  if (count === 0) {
    showToast(window._lang === 'vi' ? 'Danh sách nhân viên đang trống.' : 'The employee list is already empty.', 'info');
    return;
  }

  const firstMessage = window._lang === 'vi'
    ? `Bạn sắp xóa toàn bộ ${count} nhân viên. Dữ liệu vân tay, gán ca và đơn từ liên quan cũng sẽ bị xóa. Log chấm công thô vẫn được giữ lại.`
    : `You are about to delete all ${count} employees. Related fingerprints, shift assignments and requests will also be deleted. Raw attendance logs are retained.`;

  customConfirm(firstMessage, () => {
    const finalMessage = window._lang === 'vi'
      ? 'Xác nhận lần cuối: thao tác này không thể hoàn tác. Bạn chắc chắn muốn xóa tất cả nhân viên?'
      : 'Final confirmation: this action cannot be undone. Delete every employee?';
    customConfirm(finalMessage, deleteAllEmployees, {
      title: window._lang === 'vi' ? 'Xóa vĩnh viễn' : 'Permanent deletion',
      confirmText: window._lang === 'vi' ? 'Xóa tất cả' : 'Delete all'
    });
  }, {
    title: window._lang === 'vi' ? 'Cảnh báo xóa toàn bộ' : 'Delete-all warning',
    confirmText: window._lang === 'vi' ? 'Tiếp tục' : 'Continue'
  });
}

async function deleteAllEmployees() {
  const button = els.deleteAllEmployeesBtn;
  const originalText = button?.textContent || '';
  try {
    if (button) {
      button.disabled = true;
      button.textContent = window._lang === 'vi' ? '⏳ Đang xóa...' : '⏳ Deleting...';
    }

    const response = await api('/api/v1/employees/delete-all', {
      method: 'POST',
      body: JSON.stringify({ confirmation: 'DELETE_ALL_EMPLOYEES' })
    });
    const data = await readJsonResponse(response);
    if (!response.ok) {
      throw new Error(data?.error || data?.message || (window._lang === 'vi' ? 'Không thể xóa toàn bộ nhân viên.' : 'Failed to delete all employees.'));
    }

    state.batchSelectMode = false;
    state.batchSelected = {};
    state.selectedFingerprintEmployeeId = null;
    showToast(window._lang === 'vi'
      ? `Đã xóa ${data?.deleted || 0} nhân viên khỏi hệ thống.`
      : `Deleted ${data?.deleted || 0} employees from the system.`, 'success');
    await loadAll();
  } catch (error) {
    showToast(error?.message || (window._lang === 'vi' ? 'Xóa nhân viên thất bại.' : 'Employee deletion failed.'), 'error');
  } finally {
    if (button) {
      button.disabled = false;
      button.textContent = originalText || (window._lang === 'vi' ? '🗑 Xóa tất cả nhân viên' : '🗑 Delete all employees');
    }
  }
}

function getToolbarEnrollDeviceId() {
  const toolbarSelect = document.getElementById('stopBatchEnrollDeviceSelect');
  return toolbarSelect?.value || '';
}

async function pullAttendanceViaSDK() {
  const btn = els.loadAttendanceBtn;
  const originalText = btn?.textContent || '';
  if (btn) {
    btn.disabled = true;
    btn.textContent = window._lang === 'vi' ? '⏳ Đang lấy qua SDK...' : '⏳ Pulling via SDK...';
  }
  showToast(window._lang === 'vi'
    ? 'Đang kết nối SDK và lấy log chấm công từ các thiết bị...'
    : 'Connecting through SDK and pulling attendance logs...', 'info');
  try {
    await syncNow();
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = originalText || (window._lang === 'vi' ? 'Lấy qua SDK' : 'Pull via SDK');
    }
  }
}

async function openFingerprintModal(employeeId, autoEnroll = false) {
  state.selectedFingerprintEmployeeId = employeeId;
  const emp = state.employees.find(e => e.id === employeeId);
  if (!emp) return;

  els.fpModalEmployeeName.textContent = emp.full_name;
  els.fpModalEmployeeCode.textContent = emp.employee_code;

  // Open immediately. Loading the fingerprint list is a secondary operation;
  // a slow/offline API must not make the button appear unresponsive.
  els.fingerprintModal.classList.add('active');
  els.fingerprintModal.setAttribute('aria-busy', 'true');
  if (els.fpModalStatusBadge) {
    els.fpModalStatusBadge.textContent = window._lang === 'vi' ? 'Đang tải...' : 'Loading...';
    els.fpModalStatusBadge.className = 'badge offline';
  }
  
  els.fpInstructionBox.style.display = 'none';
  els.fpFingerprintList.style.display = 'none';
  els.fpFingerprintListItems.innerHTML = '';
  state.selectedFingerprintFingerIndex = null;
  state.fingerprintTemplates = [];

  try {
    const response = await api(`/api/v1/employees/${employeeId}/fingerprints`);
    const fps = await response.json();
    
    const hasFp = Array.isArray(fps) && fps.length > 0;
    if (hasFp) {
      state.fingerprintTemplates = fps;
      state.selectedFingerprintFingerIndex = fps[0].finger_index;
      renderFingerprintTemplates(fps);
      els.fpFingerprintList.style.display = 'block';
    } else {
      state.fingerprintTemplates = [];
      els.fpFingerprintList.style.display = 'none';
      els.fpFingerprintListItems.innerHTML = '';
    }

    // Populate các ngón tay trong select dropdown
    if (els.fpEnrollFingerIndex) {
      const trans = window.TRANSLATIONS || {};
      const fingerNames = (window._lang === 'vi' ? trans.vi?.fingerNames : trans.en?.fingerNames) || [
        'Ngón 0', 'Ngón 1', 'Ngón 2', 'Ngón 3', 'Ngón 4', 'Ngón 5', 'Ngón 6', 'Ngón 7', 'Ngón 8', 'Ngón 9'
      ];
      const registeredIndices = state.fingerprintTemplates.map(fp => fp.finger_index);
      els.fpEnrollFingerIndex.innerHTML = fingerNames.map((name, idx) => {
        const isRegistered = registeredIndices.includes(idx);
        const suffix = isRegistered ? (window._lang === 'vi' ? ' (Đã đăng ký)' : ' (Registered)') : '';
        return `<option value="${idx}">${name}${suffix}</option>`;
      }).join('');
    }
    // Single enrollment always follows the device selected in the employee
    // toolbar, so the modal cannot accidentally target another terminal.
    const toolbarDeviceId = getToolbarEnrollDeviceId();
    const toolbarDevice = state.devices.find(d => d && d.id === toolbarDeviceId);
    const enrollDevices = toolbarDevice ? [toolbarDevice] : [];
    if (hasFp) {
      els.fpModalStatusBadge.textContent = window._lang === 'vi'
        ? `Đã đăng ký (${fps.length} ngón)`
        : `Registered (${fps.length} finger${fps.length === 1 ? '' : 's'})`;
      els.fpModalStatusBadge.className = 'badge online';
      if (els.fpActionsSection) els.fpActionsSection.style.display = 'flex';
      if (els.fpReEnrollBtn) els.fpReEnrollBtn.style.display = 'inline-flex';
      if (enrollDevices.length > 0) {
        if (els.fpEnrollSection) els.fpEnrollSection.style.display = 'block';
        els.fpEnrollDeviceId.innerHTML = enrollDevices.map(d => 
          `<option value="${d.id}">${d.name} (${d.adms_enabled ? 'ADMS' : 'SDK'} - ${d.ip_address})</option>`
        ).join('');
        els.fpTriggerEnrollBtn.disabled = false;
      } else {
        if (els.fpEnrollSection) els.fpEnrollSection.style.display = 'none';
        if (els.fpReEnrollBtn) els.fpReEnrollBtn.style.display = 'none';
      }
    } else {
      els.fpModalStatusBadge.textContent = window._lang === 'vi' ? 'Chưa đăng ký' : 'Not registered';
      els.fpModalStatusBadge.className = 'badge offline';
      if (els.fpActionsSection) els.fpActionsSection.style.display = 'none';
      if (els.fpEnrollSection) els.fpEnrollSection.style.display = 'block';
      if (els.fpReEnrollBtn) els.fpReEnrollBtn.style.display = 'none';
      if (enrollDevices.length > 0) {
        els.fpEnrollDeviceId.innerHTML = enrollDevices.map(d => 
          `<option value="${d.id}">${d.name} (${d.adms_enabled ? 'ADMS' : 'SDK'} - ${d.ip_address})</option>`
        ).join('');
        els.fpTriggerEnrollBtn.disabled = false;
      } else {
        els.fpEnrollDeviceId.innerHTML = `<option value="">${window._lang === 'vi' ? '-- Không có thiết bị trực tuyến hoặc cấu hình thiếu --' : '-- No online devices available or configuration missing --'}</option>`;
        els.fpTriggerEnrollBtn.disabled = true;
      }
    }
    
    if (autoEnroll) {
      await triggerRemoteEnroll();
    }
  } catch (error) {
    showToast(error.message, 'error');
  } finally {
    els.fingerprintModal.removeAttribute('aria-busy');
  }
}

async function enrollFingerprintFromRow(employeeId) {
  await openFingerprintModal(employeeId, true);
}

async function triggerRemoteEnroll() {
  const employeeId = state.selectedFingerprintEmployeeId;
  const deviceId = getToolbarEnrollDeviceId();
  if (!deviceId) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị quét!' : 'Please select an enroll device!', 'error');
    return;
  }

  const device = state.devices.find(d => d.id === deviceId);
  const deviceName = device ? device.name : '';

  try {
    els.fpTriggerEnrollBtn.disabled = true;
    const fingerIndex = parseInt(els.fpEnrollFingerIndex ? els.fpEnrollFingerIndex.value : '0', 10);
    const response = await api(`/api/v1/employees/${employeeId}/fingerprints/enroll`, {
      method: 'POST',
      body: JSON.stringify({ device_id: deviceId, finger_index: fingerIndex })
    });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to trigger enroll');

    showToast(window._lang === 'vi'
      ? `Đã gửi yêu cầu quét tới ${deviceName || 'máy chấm công'}; đang chờ máy xác nhận và gửi dữ liệu vân tay về web.`
      : `Enrollment request queued for ${deviceName || 'biometric device'}; waiting for the device confirmation and fingerprint data.`, 'info');
    els.fpInstructionBox.style.display = 'block';
  } catch (error) {
    showToast(error.message, 'error');
    throw error;
  } finally {
    els.fpTriggerEnrollBtn.disabled = false;
  }
}

function reEnrollFingerprint(employeeId = state.selectedFingerprintEmployeeId, fingerIndex = null) {
  if (!employeeId) {
    showToast(window._lang === 'vi' ? 'Không xác định được nhân viên để đăng ký lại vân tay.' : 'Employee not specified for fingerprint re-enroll.', 'error');
    return;
  }

  const deviceId = getToolbarEnrollDeviceId();
  if (!deviceId) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị để đăng ký lại!' : 'Please select a device to re-enroll!', 'error');
    return;
  }

  if (fingerIndex === null || fingerIndex === undefined) {
    fingerIndex = parseInt(els.fpEnrollFingerIndex ? els.fpEnrollFingerIndex.value : '0', 10);
  }

  state.selectedFingerprintEmployeeId = employeeId;
  const employee = state.employees.find(e => e.id === employeeId);
  const device = state.devices.find(d => d.id === deviceId);
  const employeeName = employee?.full_name || employee?.employee_code || employeeId;
  const deviceName = device?.name || (window._lang === 'vi' ? 'máy chấm công đã chọn' : 'selected device');
  
  const trans = window.TRANSLATIONS || {};
  const fingerNames = (window._lang === 'vi' ? trans.vi?.fingerNames : trans.en?.fingerNames) || [];
  const fingerName = fingerNames[fingerIndex] || `Ngón #${fingerIndex}`;
  
  const message = window._lang === 'vi'
    ? `Đăng ký lại ${fingerName} cho "${employeeName}" trên "${deviceName}"? Mẫu cũ trên hệ thống chỉ được thay thế sau khi máy nhận mẫu mới thành công.`
    : `Re-enroll ${fingerName} for "${employeeName}" on "${deviceName}"? The saved template is replaced only after a new capture succeeds.`;

  customConfirm(message, () => submitFingerprintReEnroll(employeeId, deviceId, fingerIndex), {
    title: window._lang === 'vi' ? 'Đăng ký lại vân tay' : 'Re-enroll fingerprint',
    confirmText: window._lang === 'vi' ? 'Bắt đầu quét' : 'Start scan'
  });
}

function reEnrollFingerprintFromRow(employeeId) {
  reEnrollFingerprint(employeeId);
}

async function submitFingerprintReEnroll(employeeId, deviceId, fingerIndex = 0) {
  try {
    if (els.fpReEnrollBtn) els.fpReEnrollBtn.disabled = true;
    const response = await api(`/api/v1/employees/${employeeId}/fingerprints/re-enroll`, {
      method: 'POST',
      body: JSON.stringify({ device_id: deviceId, finger_index: fingerIndex })
    });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to trigger re-enroll');

    const device = state.devices.find(d => d.id === deviceId);
    const deviceName = device?.name || '';
    showToast(window._lang === 'vi'
      ? `Đã gửi yêu cầu đăng ký lại tới ${deviceName || 'máy chấm công'}; đang chờ máy xác nhận và gửi dữ liệu vân tay về web.`
      : `Re-enrollment request queued for ${deviceName || 'biometric device'}; waiting for the device confirmation and fingerprint data.`, 'info');
    if (els.fpInstructionBox && els.fingerprintModal?.classList.contains('active')) {
      els.fpInstructionBox.style.display = 'block';
    }
  } catch (error) {
    showToast(error.message, 'error');
  } finally {
    if (els.fpReEnrollBtn) els.fpReEnrollBtn.disabled = false;
  }
}

function renderFingerprintTemplates(fps) {
  if (!els.fpFingerprintListItems) return;
  const trans = window.TRANSLATIONS || {};
  const fingerNames = (window._lang === 'vi' ? trans.vi?.fingerNames : trans.en?.fingerNames) || [];

  els.fpFingerprintListItems.innerHTML = fps.map(fp => {
    const created = fp.created_at ? ` • ${new Date(fp.created_at).toLocaleString('vi-VN')}` : '';
    const fingerName = fingerNames[fp.finger_index] || `Ngón #${fp.finger_index}`;
    return `
      <div class="fp-item" style="display:flex; justify-content:space-between; align-items:center; gap: 10px; padding:10px; border:1px solid var(--border); border-radius:8px; background: rgba(255,255,255,0.02);">
        <div>
          <div style="font-size:0.95rem; font-weight:600;">${fingerName}</div>
          <div style="font-size:0.85rem; color: var(--muted);">Kích thước: ${fp.template_size} bytes${created}</div>
        </div>
        <div style="display: flex; gap: 6px;">
          <button type="button" class="primary-btn" style="padding: 6px 10px; font-size:0.8rem; background: var(--accent); border-color: var(--accent);" onclick="reEnrollFingerprint('${state.selectedFingerprintEmployeeId}', ${fp.finger_index})" title="${window._lang === 'vi' ? 'Đăng ký lại ngón này' : 'Re-enroll this finger'}">
            🔁
          </button>
          <button type="button" class="secondary-btn" style="padding: 6px 10px; font-size:0.8rem; color: var(--danger); border-color: var(--danger);" onclick="deleteFingerprint(${fp.finger_index})" title="${window._lang === 'vi' ? 'Xoá ngón này' : 'Delete this finger'}">
            🗑️
          </button>
        </div>
      </div>`;
  }).join('');
}

async function pushFingerprintsToAll() {
  const employeeId = state.selectedFingerprintEmployeeId;
  try {
    const response = await api(`/api/v1/employees/${employeeId}/fingerprints/push`, { method: 'POST' });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to push fingerprints');
    
    showToast(window._lang === 'vi' ? 'Đã đặt lịch đẩy vân tay sang các thiết bị khác!' : 'Queued fingerprint propagation to other devices!', 'success');
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function deleteFingerprint(fingerIndex = null) {
  const employeeId = state.selectedFingerprintEmployeeId;
  if (!employeeId) {
    showToast(window._lang === 'vi' ? 'Không xác định được nhân viên để xóa vân tay.' : 'Employee not specified for fingerprint delete.', 'error');
    return;
  }

  if (fingerIndex === null || fingerIndex === undefined) {
    fingerIndex = state.selectedFingerprintFingerIndex;
  }

  if (fingerIndex === null || fingerIndex === undefined) {
    try {
      const response = await api(`/api/v1/employees/${employeeId}/fingerprints`);
      const fps = await response.json();
      if (!Array.isArray(fps) || fps.length === 0) {
        showToast(window._lang === 'vi' ? 'Không có vân tay nào để xóa.' : 'No fingerprint templates found to delete.', 'error');
        return;
      }
      if (fps.length === 1) {
        fingerIndex = fps[0].finger_index;
      } else {
        showToast(window._lang === 'vi' ? 'Nhân viên có nhiều mẫu vân tay, vui lòng xóa trong modal.' : 'Employee has multiple fingerprint templates, please delete from the modal.', 'warning');
        openFingerprintModal(employeeId);
        return;
      }
    } catch (error) {
      showToast(error.message || (window._lang === 'vi' ? 'Không thể lấy danh sách vân tay.' : 'Failed to fetch fingerprint list.'), 'error');
      return;
    }
  }

  const confirmMsg = window._lang === 'vi'
    ? `Bạn có chắc chắn muốn xoá vân tay ngón #${fingerIndex + 1} của nhân viên này? Lệnh xoá cũng sẽ được gửi xuống các thiết bị.`
    : `Are you sure you want to delete fingerprint index #${fingerIndex} for this employee? Delete command will also be sent to devices.`;

  customConfirm(confirmMsg, async () => {
    try {
      const response = await api(`/api/v1/employees/${employeeId}/fingerprints/${fingerIndex}`, { method: 'DELETE' });
      const data = await readJsonResponse(response);
      if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to delete fingerprint');

      showToast(window._lang === 'vi' ? 'Đã xoá vân tay thành công!' : 'Fingerprint deleted successfully!', 'success');
      state.selectedFingerprintFingerIndex = null;
      state.fingerprintTemplates = [];
      openFingerprintModal(employeeId);
      loadAll();
    } catch (error) {
      showToast(error.message, 'error');
    }
  });
}

async function testDeviceConnection(deviceId) {
  try {
    showToast(window._lang === 'vi' ? 'Đang kiểm tra kết nối thiết bị...' : 'Testing device connection...', 'info');
    const response = await api(`/api/v1/devices/${deviceId}/test-connection`, { method: 'POST' });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to test device');
    showToast(window._lang === 'vi' ? `Kết nối thiết bị OK: ${data?.online ? 'Online' : 'Offline'}` : `Device connection OK: ${data?.online ? 'Online' : 'Offline'}`, data?.online ? 'success' : 'warning');
    await loadAll();
  } catch (error) {
    showToast(error?.message || 'Failed to test device', 'error');
  }
}

async function rebootDevice(deviceId) {
  try {
    showToast(window._lang === 'vi' ? 'Đang gửi lệnh reboot thiết bị...' : 'Sending reboot command...', 'info');
    const response = await api(`/api/v1/devices/${deviceId}/reboot`, { method: 'POST' });
    if (!response.ok) {
      const data = await readJsonResponse(response);
      throw new Error(data?.error || data?.message || 'Failed to reboot device');
    }
    showToast(window._lang === 'vi' ? 'Đã gửi lệnh reboot thiết bị.' : 'Reboot command sent.', 'success');
  } catch (error) {
    showToast(error?.message || 'Failed to reboot device', 'error');
  }
}

async function pullDeviceEmployees(deviceId) {
  try {
    showToast(window._lang === 'vi' ? 'Đang kéo danh sách nhân viên từ máy...' : 'Pulling employees from device...', 'info');
    const response = await api(`/api/v1/devices/${deviceId}/pull-employees`, { method: 'POST' });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to pull employees');
    showToast(window._lang === 'vi' ? `Đã kéo xong: ${data?.imported || 0} mới, ${data?.existing || 0} đã có.` : `Pull completed: ${data?.imported || 0} new, ${data?.existing || 0} existing.`, 'success');
    await loadAll();
  } catch (error) {
    showToast(error?.message || 'Failed to pull employees', 'error');
  }
}

async function syncDeviceEmployees(deviceId) {
  try {
    showToast(window._lang === 'vi' ? 'Đang đồng bộ nhân viên xuống thiết bị...' : 'Syncing employees to device...', 'info');
    const response = await api(`/api/v1/devices/${deviceId}/sync-employees`, { method: 'POST' });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to sync employees');
    showToast(window._lang === 'vi' ? 'Đã gửi lệnh đồng bộ nhân viên xuống thiết bị.' : 'Employee sync command sent.', 'success');
    await loadAll();
  } catch (error) {
    showToast(error?.message || 'Failed to sync employees', 'error');
  }
}

async function clearDeviceLogs(deviceId) {
  try {
    const device = state.devices.find(d => d.id === deviceId);
    if (!device) return;
    showConfirm(
      window._lang === 'vi' 
        ? `Xác nhận xóa toàn bộ log chấm công của thiết bị "${device.name}"? Thao tác này không thể hoàn tác.`
        : `Confirm clearing all attendance logs from device "${device.name}"? This action cannot be undone.`,
      async () => {
        try {
          showToast(window._lang === 'vi' ? `Đang gửi lệnh xóa log...` : `Sending clear logs command...`, 'info');
          const response = await api(`/api/v1/devices/${deviceId}/clear-logs`, { method: 'POST' });
          const data = await readJsonResponse(response);
          if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to clear logs');
          showToast(window._lang === 'vi' ? 'Lệnh xóa log đã được gửi tới thiết bị.' : 'Clear logs command sent.', 'success');
          await loadAll();
        } catch (error) {
          showToast(error?.message || 'Failed to clear logs', 'error');
        }
      }
    );
  } catch (error) {
    showToast(error?.message || 'Failed to clear logs', 'error');
  }
}

async function resetDevice(deviceId) {
  try {
    const device = state.devices.find(d => d.id === deviceId);
    if (!device) return;
    showConfirm(
      window._lang === 'vi' 
        ? `⚠️ CẢNH BÁO: Xác nhận RESET toàn bộ thiết bị "${device.name}"? Điều này sẽ xóa TẤT CẢ nhân viên, vân tay, và log chấm công. Thao tác không thể hoàn tác!`
        : `⚠️ WARNING: Confirm RESET entire device "${device.name}"? This will delete ALL employees, fingerprints, and attendance logs. This action cannot be undone!`,
      async () => {
        try {
          showToast(window._lang === 'vi' ? `Đang gửi lệnh reset thiết bị...` : `Sending device reset command...`, 'info');
          const response = await api(`/api/v1/devices/${deviceId}/reset`, { method: 'POST' });
          const data = await readJsonResponse(response);
          if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to reset device');
          showToast(window._lang === 'vi' ? 'Lệnh reset thiết bị đã được gửi. Thiết bị sẽ xóa tất cả dữ liệu.' : 'Device reset command sent. Device will clear all data.', 'success');
          await loadAll();
        } catch (error) {
          showToast(error?.message || 'Failed to reset device', 'error');
        }
      }
    );
  } catch (error) {
    showToast(error?.message || 'Failed to reset device', 'error');
  }
}


async function onTableAction(event) {
  const target = event.target.closest('button, a');
  if (!target || !target.dataset.action) return;
  
  const action = target.dataset.action;
  const id = target.dataset.id;
  const type = target.dataset.type;

  switch (action) {
    case 'edit-device':
      editDevice(id);
      break;
    case 'test-device':
      testDeviceConnection(id);
      break;
    case 'reboot-device':
      rebootDevice(id);
      break;
    case 'pull-device-employees':
      pullDeviceEmployees(id);
      break;
    case 'sync-device-employees':
      syncDeviceEmployees(id);
      break;
    case 'clear-device-logs':
      clearDeviceLogs(id);
      break;
    case 'reset-device':
      resetDevice(id);
      break;
    case 'backup-device':
      openBackupDeviceModal(id);
      break;
    case 'delete-device':
      customConfirm(
        window._lang === 'vi' ? 'Bạn có chắc chắn muốn xoá thiết bị này?' : 'Are you sure you want to delete this device?',
        () => deleteDevice(id)
      );
      break;
    case 'map-employee-device':
      syncEmployeeToDevice(id);
      break;
    case 'confirm-fingerprint':
      openFingerprintModal(id);
      break;
    case 'edit-employee':
      editEmployee(id);
      break;
    case 'delete-employee':
      customConfirm(
        window._lang === 'vi' ? 'Bạn có chắc chắn muốn xoá nhân viên này?' : 'Are you sure you want to delete this employee?',
        () => deleteEmployee(id)
      );
      break;
    case 'delete-shift':
      customConfirm(
        window._lang === 'vi' ? 'Bạn có chắc chắn muốn xoá ca làm việc này?' : 'Are you sure you want to delete this shift?',
        () => deleteShift(id)
      );
      break;
    case 'approve-ess':
      handleEssApproval('approve', type, id);
      break;
    case 'reject-ess':
      handleEssApproval('reject', type, id);
      break;
    case 'push-to-all-devices':
      pushEmployeeToAllDevices(id);
      break;
  }
}

function renderEmployees() {
  const searchTerm = els.employeeSearch.value.trim().toLowerCase();
  const filtered = state.employees.filter((emp) => 
    (emp.full_name || '').toLowerCase().includes(searchTerm) || 
    (emp.employee_code || '').toLowerCase().includes(searchTerm) ||
    (emp.department_id && emp.department_id.toLowerCase().includes(searchTerm)) ||
    (emp.job_title && emp.job_title.toLowerCase().includes(searchTerm))
  );

  els.employeeTableBody.innerHTML = filtered.map((employee) => {
    const statusClass = (employee.status || 'active') === 'active' ? 'online' : 'offline';
    const statusText = (employee.status || 'active') === 'active' ? t('optStatusActive') : t('optStatusInactive');
    
    // Avatar html
    const firstChar = (employee.full_name || '?')[0].toUpperCase();
    const avatarHtml = employee.avatar_url ? 
      `<img src="${employee.avatar_url}" style="width:36px; height:36px; border-radius:50%; margin-right:12px; object-fit:cover; border:2px solid var(--border);" />` : 
      `<div style="width:36px; height:36px; border-radius:50%; margin-right:12px; background:var(--accent-gradient); display:grid; place-items:center; font-weight:600; color:white; font-size:0.9rem;">${firstChar}</div>`;
    
    const infoHtml = `
      <div style="display:flex; align-items:center;">
        ${avatarHtml}
        <div>
          <strong style="font-size:0.95rem;">${employee.full_name}</strong>
          <div class="muted" style="font-size:0.8rem; margin-top:2px;">${employee.email || (window._lang === 'vi' ? 'Không có email' : 'No email')} | ${employee.phone || (window._lang === 'vi' ? 'Không có SĐT' : 'No phone')}</div>
        </div>
      </div>
    `;
    
    const deptTitleHtml = `
      <div>
        <strong>${employee.job_title || (window._lang === 'vi' ? 'Nhân viên' : 'Employee')}</strong>
        <div class="muted" style="font-size:0.8rem; margin-top:2px;">${employee.department_id || (window._lang === 'vi' ? 'Chưa phân phòng' : 'No department')}</div>
      </div>
    `;

    const joinDateText = employee.join_date ? new Date(employee.join_date).toLocaleDateString(window._lang === 'vi' ? 'vi-VN' : 'en-US') : '-';

    // Badge vân tay & khuôn mặt
    const fpEnrolled = employee.fingerprint_enrolled;
    const faceEnrolled = employee.face_enrolled;
    const fpBadgeHtml = fpEnrolled
      ? `<span class="badge online" style="font-size:0.75rem; padding: 2px 6px;">🖐 Vân tay: Có</span>`
      : `<span class="badge offline" style="font-size:0.75rem; padding: 2px 6px;">⚠ Vân tay: Chưa</span>`;
    const faceBadgeHtml = faceEnrolled
      ? `<span class="badge online" style="font-size:0.75rem; padding: 2px 6px; background: rgba(16,185,129,0.15); color: #10B981; border: 1px solid #10B981;">📷 Mặt: Có</span>`
      : `<span class="badge offline" style="font-size:0.75rem; padding: 2px 6px;">⚠ Mặt: Chưa</span>`;

    const bioBadgesHtml = `<div style="display:flex; flex-direction:column; gap:3px;">${fpBadgeHtml}${faceBadgeHtml}</div>`;

    return `
      <tr>
        ${state.batchSelectMode ? `<td style="width:40px; text-align:center;"><input type="checkbox" onchange="toggleEmployeeSelection('${employee.id}', this.checked)" ${state.batchSelected[employee.id] ? 'checked' : ''} /></td>` : ''}
        <td><code>${employee.employee_code}</code></td>
        <td>${infoHtml}</td>
        <td>${deptTitleHtml}</td>
        <td><span class="muted" style="font-size:0.9rem;">${joinDateText}</span></td>
        <td>${bioBadgesHtml}</td>
        <td><span class="badge ${statusClass}">${statusText}</span></td>
        <td>
          <div class="row-dropdown" style="display:flex; align-items:center;">
            <button type="button" class="row-dropdown-btn" onclick="toggleRowDropdown(event, '${employee.id}')">⋮</button>
            <div class="row-dropdown-content" id="dropdown-${employee.id}">
              <button type="button" onclick="event.stopPropagation(); this.parentElement.classList.remove('active'); openRegisterFaceModal('${employee.id}');">📷 ${faceEnrolled ? (window._lang === 'vi' ? 'Đăng ký lại khuôn mặt' : 'Re-register face') : (window._lang === 'vi' ? 'Đăng ký khuôn mặt' : 'Register face')}</button>
              ${faceEnrolled ? `<button type="button" onclick="event.stopPropagation(); this.parentElement.classList.remove('active'); deleteFaceFromRow('${employee.id}');">🗑️ ${window._lang === 'vi' ? 'Xóa khuôn mặt' : 'Delete face'}</button>` : ''}
              <button type="button" onclick="event.stopPropagation(); this.parentElement.classList.remove('active'); state.selectedFingerprintEmployeeId='${employee.id}'; openFingerprintModal('${employee.id}');">🖐️ ${window._lang === 'vi' ? 'Quản lý vân tay' : 'Manage fingerprint'}</button>
              <button type="button" onclick="event.stopPropagation(); this.parentElement.classList.remove('active'); state.selectedFingerprintEmployeeId='${employee.id}'; enrollFingerprintFromRow('${employee.id}');">⚡ ${window._lang === 'vi' ? 'Đăng ký vân tay' : 'Enroll fingerprint'}</button>
              <button type="button" onclick="event.stopPropagation(); this.parentElement.classList.remove('active'); reEnrollFingerprintFromRow('${employee.id}');">🔁 ${window._lang === 'vi' ? 'Đăng ký lại vân tay' : 'Re-enroll fingerprint'}</button>
              <button type="button" onclick="event.stopPropagation(); this.parentElement.classList.remove('active'); state.selectedFingerprintEmployeeId='${employee.id}'; deleteFingerprint();">🗑️ ${window._lang === 'vi' ? 'Xóa vân tay' : 'Delete fingerprint'}</button>
              <button type="button" onclick="event.stopPropagation(); this.parentElement.classList.remove('active'); state.selectedFingerprintEmployeeId='${employee.id}'; pushFingerprintsToAll();">🔄 ${window._lang === 'vi' ? 'Đồng bộ vân tay' : 'Push fingerprints'}</button>
              <button type="button" onclick="event.stopPropagation(); this.parentElement.classList.remove('active'); clearEmployeeEnrollData('${employee.id}');">🗑️ ${window._lang === 'vi' ? 'Xóa dữ liệu vân tay' : 'Clear fingerprint data'}</button>
              <button type="button" onclick="event.stopPropagation(); this.parentElement.classList.remove('active'); editEmployee('${employee.id}');">✏️ ${t('btnEdit')}</button>
              <button type="button" onclick="event.stopPropagation(); this.parentElement.classList.remove('active'); showToast(window._lang === 'vi' ? 'Đang mở hộp xác nhận xoá...' : 'Opening delete confirmation...', 'info'); deleteEmployeePrompt('${employee.id}');" style="color: var(--danger); font-weight: 500;">🗑️ ${t('btnDelete')}</button>
            </div>
          </div>
        </td>
      </tr>`;
  }).join('');
}

// Toggle inline selection mode or perform enroll when already selecting
function toggleSelectEnroll() {
  const btn = document.getElementById('batchEnrollBtn');
  if (!state.batchSelectMode) {
    state.batchSelectMode = true;
    state.batchSelected = {};
    if (btn) btn.innerHTML = '🖐 Bắt đầu quét';
    renderEmployees();
    showToast(window._lang === 'vi' ? 'Chọn các nhân viên cần quét, rồi bấm lại để bắt đầu.' : 'Select employees then click again to start enroll.', 'info');
    return;
  }

  // Collect selected employees
  const selected = Object.keys(state.batchSelected).filter(k => state.batchSelected[k]);
  if (selected.length === 0) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn ít nhất 1 nhân viên để quét.' : 'Please select at least 1 employee to enroll.', 'warning');
    return;
  }

  const deviceSelect = document.getElementById('stopBatchEnrollDeviceSelect');
  const deviceId = deviceSelect ? deviceSelect.value : '';
  if (!deviceId) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị đích trên thanh công cụ.' : 'Please select target device in toolbar.', 'warning');
    return;
  }

  // Exit select mode while processing
  state.batchSelectMode = false;
  if (btn) btn.innerHTML = '🖐 Chọn để quét';
  renderEmployees();

  (async () => {
    try {
      showToast(window._lang === 'vi' ? `Đang gửi ${selected.length} lệnh quét...` : `Sending ${selected.length} enroll commands...`, 'info');
      const response = await api('/api/v1/employees/batch-enroll', {
        method: 'POST',
        body: JSON.stringify({ employee_ids: selected, device_id: deviceId })
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data?.error || data?.message || 'Failed');
      showToast(window._lang === 'vi' ? `Đã đưa ${data.enqueued || 0}/${data.total_requested || selected.length} lệnh vào queue` : `Queued ${data.enqueued || 0}/${data.total_requested || selected.length} commands`, 'success');
      state.batchSelected = {};
      renderEmployees();
    } catch (err) {
      showToast(err.message || 'Failed to enqueue', 'error');
    }
  })();
}

function toggleEmployeeSelection(empID, checked) {
  if (checked) state.batchSelected[empID] = true;
  else delete state.batchSelected[empID];
}

function renderAttendance() {
  els.attendanceSummaryBody.innerHTML = state.attendanceSummary.length ? state.attendanceSummary.map((item) => {
    // Map employee detail from state
    const employee = state.employees.find(emp => emp.id === item.employee_id || emp.employee_code === item.employee_id);
    const fullName = employee ? employee.full_name : (item.full_name || item.employee_id);
    const code = employee ? employee.employee_code : (item.employee_code || '');

    // Map shift detail from state
    const shift = state.shifts.find(s => s.id === item.shift_id);
    const shiftBadge = shift ? `<br/><span class="badge" style="background: ${shift.color_code || 'var(--accent)'}; color: white; margin-top:4px; font-size:0.75rem; padding: 2px 6px;">${shift.name}</span>` : `<br/><span class="badge" style="background:#475569; color:#cbd5e1; margin-top:4px; font-size:0.75rem; padding: 2px 6px;">${t('freePunch')}</span>`;

    const dateFormatted = new Date(item.date).toLocaleDateString(window._lang === 'vi' ? 'vi-VN' : 'en-US');

    // Check-in and check-out times
    const checkInTime = item.first_in ? new Date(item.first_in).toLocaleTimeString(window._lang === 'vi' ? 'vi-VN' : 'en-US', {hour: '2-digit', minute:'2-digit'}) : '---';
    const checkOutTime = item.last_out ? new Date(item.last_out).toLocaleTimeString(window._lang === 'vi' ? 'vi-VN' : 'en-US', {hour: '2-digit', minute:'2-digit'}) : '---';
    const checkInOutHtml = `
      <div>
        <span class="badge online">${checkInTime}</span>
        <span class="muted">/</span>
        <span class="badge offline">${checkOutTime}</span>
      </div>
    `;

    // Grace / Late / Early
    const lateDisplay = item.late_minutes > 0 ? `<div style="color: var(--warning); font-size:0.85rem; font-weight:500;">${t('late')}: ${item.late_minutes} ${window._lang === 'vi' ? 'ph' : 'min'}</div>` : '';
    const earlyDisplay = item.early_minutes > 0 ? `<div style="color: var(--warning); font-size:0.85rem; font-weight:500;">${window._lang === 'vi' ? 'Sớm' : 'Early'}: ${item.early_minutes} ${window._lang === 'vi' ? 'ph' : 'min'}</div>` : '';
    const graceDisplay = (lateDisplay || earlyDisplay) ? `${lateDisplay}${earlyDisplay}` : `<span style="color: var(--success); font-size:0.85rem;">${window._lang === 'vi' ? 'Đúng giờ' : 'On time'}</span>`;

    // Overtime
    const otDisplay = item.overtime_minutes > 0 ? `<strong style="color: #38bdf8; font-size:0.9rem;">+${item.overtime_minutes} ${window._lang === 'vi' ? 'ph' : 'min'}</strong>` : '<span class="muted">---</span>';
    const regularMinutes = Number.isFinite(Number(item.regular_working_minutes))
      ? Number(item.regular_working_minutes)
      : Math.round(Math.max(0, (Number(item.working_hours) || 0) * 60 - (Number(item.overtime_minutes) || 0)));
    const regularHoursDisplay = `<span style="font-weight:600;">${(regularMinutes / 60).toFixed(2)}</span> ${window._lang === 'vi' ? 'giờ' : 'h'}`;

    // Leave status
    const leaveDisplay = item.leave_id ? `<span class="badge" style="background: #3b82f6; color: white; font-size:0.75rem;">${t('leave')}</span>` : '<span class="muted">---</span>';

    // Total Attendance status badge
    let statusClass = 'absent';
    let statusText = t('absent');
    if (item.attendance_status === 'present') {
      statusClass = 'present';
      statusText = t('present');
    } else if (item.attendance_status === 'late') {
      statusClass = 'partial';
      statusText = t('late');
    } else if (item.attendance_status === 'early') {
      statusClass = 'partial';
      statusText = window._lang === 'vi' ? 'Về sớm' : 'Early';
    } else if (item.attendance_status === 'leave') {
      statusClass = 'info';
      statusText = t('leave');
    }

    return `
      <tr>
        <td><strong>${fullName}</strong> <span class="muted">(${code})</span></td>
        <td><code>${dateFormatted}</code>${shiftBadge}</td>
        <td>${checkInOutHtml}</td>
        <td>${graceDisplay}</td>
        <td>${regularHoursDisplay}</td>
        <td>${otDisplay}</td>
        <td>${leaveDisplay}</td>
        <td><span class="badge ${statusClass}">${statusText}</span></td>
      </tr>`;
  }).join('') : `<tr><td colspan="8">${window._lang === 'vi' ? 'Không có dữ liệu tóm tắt ngày hôm nay' : 'No summary data for today'}</td></tr>`;

  els.attendanceTableBody.innerHTML = state.attendance.map((item) => {
    let typeClass = 'partial';
    let typeText = t('cardScan');
    if (item.check_type === 'in') {
      typeClass = 'online';
      typeText = 'Check In';
    } else if (item.check_type === 'out') {
      typeClass = 'offline';
      typeText = 'Check Out';
    }
    const empDisplay = item.employee_name ? `<strong>${item.employee_name}</strong> <span class="muted" style="font-size:0.85rem;">(${item.employee_code})</span>` : `<code>${item.employee_code}</code>`;
    const devDisplay = item.device_name ? `<strong>${item.device_name}</strong>` : `<code class="muted">${item.device_id}</code>`;
    const isValid = item.is_valid !== false;
    let validityDisplay = `<span class="badge online">${t('logWithinShift')}</span>`;
    if (!isValid) {
      let reason = item.invalid_reason || t('logInvalidUnknown');
      if (reason === 'No shift assigned') reason = t('logReasonNoShift');
      if (reason === 'Outside shift window +-1h' || reason === 'Outside shift window and no approved overtime') reason = t('logReasonOutsideWindow');
      let segmentLabel = item.work_segment === 'overtime' ? t('logOvertime') : t('logOutsideShift');
      validityDisplay = `<span class="badge offline">${segmentLabel}: ${reason}</span>`;
    }
    if (isValid && item.work_segment === 'overtime') {
      validityDisplay = `<span class="badge" style="background:#0e7490;color:#cffafe;">${t('logOvertime')}</span>`;
    }
    return `
      <tr>
        <td>${empDisplay}</td>
        <td><code>${new Date(item.check_time).toLocaleString(window._lang === 'vi' ? 'vi-VN' : 'en-US')}</code></td>
        <td><span class="badge ${typeClass}">${typeText}</span></td>
        <td>${devDisplay}</td>
        <td>${validityDisplay}</td>
      </tr>`;
  }).join('');
}

function switchAttendanceSection(section) {
  const showSummary = section !== 'raw';
  els.attendanceSummarySection?.classList.toggle('hidden', !showSummary);
  els.attendanceRawSection?.classList.toggle('hidden', showSummary);
  els.showAttendanceSummaryBtn?.classList.toggle('active', showSummary);
  els.showAttendanceRawBtn?.classList.toggle('active', !showSummary);
  els.showAttendanceSummaryBtn?.setAttribute('aria-selected', String(showSummary));
  els.showAttendanceRawBtn?.setAttribute('aria-selected', String(!showSummary));
}

function renderSyncHistory() {
  els.syncTableBody.innerHTML = state.syncHistory.map((item) => {
    const statusClass = item.status === 'success' ? 'online' : 'offline';
    const statusText = item.status === 'success' ? t('success') : t('failed');
    return `
      <tr>
        <td><strong>${item.device_id || t('thAttDevice')}</strong></td>
        <td>${item.sync_type.toUpperCase()}</td>
        <td>${item.trigger_type.toUpperCase()}</td>
        <td><span class="badge ${statusClass}">${statusText}</span></td>
        <td><span style="font-weight:600;">${item.record_count}</span></td>
      </tr>`;
  }).join('');
}

async function loadShifts() {
  try {
    const [resShifts, resRotations, resEmpShifts, resSwaps] = await Promise.all([
      api('/api/v1/shifts'),
      api('/api/v1/rotation-patterns'),
      api('/api/v1/employee-shifts'),
      api('/api/v1/shift-swaps')
    ]);

    state.shifts = resShifts.ok ? (await resShifts.json() || []) : [];
    state.rotationPatterns = resRotations.ok ? (await resRotations.json() || []) : [];
    state.employeeShifts = resEmpShifts.ok ? (await resEmpShifts.json() || []) : [];
    state.shiftSwaps = resSwaps.ok ? (await resSwaps.json() || []) : [];
  } catch (err) {
    console.error('Error loading shift management data:', err);
    state.shifts = state.shifts || [];
    state.rotationPatterns = state.rotationPatterns || [];
    state.employeeShifts = state.employeeShifts || [];
    state.shiftSwaps = state.shiftSwaps || [];
  }
}

function toggleAssignFields() {
  const type = els.assignType.value;
  if (type === 'shift') {
    els.assignShiftIdLabel.style.display = 'block';
    els.assignRotationPatternIdLabel.style.display = 'none';
  } else {
    els.assignShiftIdLabel.style.display = 'none';
    els.assignRotationPatternIdLabel.style.display = 'block';
  }
}

function toggleBatchAssignFields() {
  const target = els.batchAssignTargetType.value;
  if (target === 'all') {
    els.batchAssignDeptLabel.style.display = 'none';
    els.batchAssignEmployeesLabel.style.display = 'none';
  } else if (target === 'department') {
    els.batchAssignDeptLabel.style.display = 'block';
    els.batchAssignEmployeesLabel.style.display = 'none';
  } else {
    els.batchAssignDeptLabel.style.display = 'none';
    els.batchAssignEmployeesLabel.style.display = 'block';
  }

  const type = els.batchAssignType.value;
  if (type === 'shift') {
    els.batchAssignShiftIdLabel.style.display = 'block';
    els.batchAssignRotationPatternIdLabel.style.display = 'none';
  } else {
    els.batchAssignShiftIdLabel.style.display = 'none';
    els.batchAssignRotationPatternIdLabel.style.display = 'block';
  }
}

function addRotationStep() {
  const stepIdx = els.rotationStepsContainer.children.length + 1;
  const div = document.createElement('div');
  div.className = 'rotation-step-row';
  div.style = 'display: flex; gap: 10px; align-items: center; margin-bottom: 5px;';
  
  const options = state.shifts.map(s => `<option value="${s.id}">${s.name}</option>`).join('');
  div.innerHTML = `
    <span style="font-size: 0.9rem; font-weight: 500; min-width: 60px;">Bước ${stepIdx}:</span>
    <select class="form-input rotation-shift-select" style="flex-grow: 1; height: 38px;" required>
      ${options}
    </select>
    <input type="number" class="form-input rotation-duration-input" placeholder="Số ngày" min="1" value="1" style="width: 85px; height: 38px;" required />
    <button type="button" class="ghost-btn" style="color: var(--danger); border-color: var(--danger); padding: 4px 8px; height: 38px;" onclick="this.parentElement.remove(); reindexRotationSteps();">✕</button>
  `;
  els.rotationStepsContainer.appendChild(div);
}

function reindexRotationSteps() {
  Array.from(els.rotationStepsContainer.children).forEach((child, idx) => {
    const span = child.querySelector('span');
    if (span) span.textContent = `Bước ${idx + 1}:`;
  });
}

function renderShifts() {
  // Defensive initialization of state arrays
  state.shifts = state.shifts || [];
  state.rotationPatterns = state.rotationPatterns || [];
  state.employeeShifts = state.employeeShifts || [];
  state.shiftSwaps = state.shiftSwaps || [];
  state.employees = state.employees || [];

  // Render Shift Table
  els.shiftTableBody.innerHTML = state.shifts.map((s) => `
    <tr>
      <td><strong>${s.name}</strong></td>
      <td>${s.start_time ? s.start_time.substring(0, 5) : ''}</td>
      <td>${s.end_time ? s.end_time.substring(0, 5) : ''}</td>
      <td>
        <button class="secondary-btn" style="margin-right: 5px;" onclick="editShift('${s.id}')">${t('btnEdit')}</button>
        <button class="secondary-btn" style="color: var(--danger); border-color: var(--danger);" onclick="deleteShift('${s.id}')">${t('btnDelete')}</button>
      </td>
    </tr>`).join('');

  // Render Rotation Table
  els.rotationTableBody.innerHTML = state.rotationPatterns.map((rp) => {
    let sequenceText = "";
    try {
      const seqStr = rp.pattern_sequence || rp.pattern || "[]";
      const parsed = typeof seqStr === 'string' ? JSON.parse(seqStr) : seqStr;
      sequenceText = (parsed || []).map(step => {
        const sName = state.shifts.find(s => s.id === step.shift_id)?.name || step.shift_id;
        return `${sName} (${step.duration} ${window._lang === 'vi' ? 'ngày' : 'days'})`;
      }).join(' ➔ ');
    } catch (e) {
      console.error('Error parsing rotation pattern sequence:', e);
      sequenceText = rp.pattern_sequence || rp.pattern || "";
    }
    return `
      <tr>
        <td><strong>${rp.name}</strong></td>
        <td style="font-size: 0.85rem; color: var(--muted);">${sequenceText}</td>
        <td>
          <button class="secondary-btn" style="margin-right: 5px;" onclick="editRotationPattern('${rp.id}')">${t('btnEdit')}</button>
          <button class="secondary-btn" style="color: var(--danger); border-color: var(--danger);" onclick="deleteRotationPattern('${rp.id}')">${t('btnDelete')}</button>
        </td>
      </tr>`;
  }).join('');

  // Render Employee Shift Assignments with filtering
  const selectedFilter = state.selectedShiftFilter || 'all';
  const filteredAssignments = state.employeeShifts.filter(es => {
    if (selectedFilter === 'all') return true;
    return es.shift_id === selectedFilter || es.rotation_pattern_id === selectedFilter;
  });

  els.employeeShiftTableBody.innerHTML = filteredAssignments.map((es) => {
    const emp = state.employees.find(e => e.id === es.employee_id);
    const empName = emp ? emp.full_name : es.employee_id;
    const empCode = emp ? emp.employee_code : 'N/A';
    
    let assignDetail = "";
    if (es.rotation_pattern_id) {
      const rot = state.rotationPatterns.find(r => r.id === es.rotation_pattern_id);
      assignDetail = `<span class="badge" style="background: var(--success); color: white;">${window._lang === 'vi' ? 'Chu kỳ' : 'Rotation'}: ${rot ? rot.name : es.rotation_pattern_id}</span>`;
    } else if (es.shift_id) {
      const s = state.shifts.find(sh => sh.id === es.shift_id);
      assignDetail = `<span class="badge" style="background: var(--primary); color: white;">${window._lang === 'vi' ? 'Cố định' : 'Fixed'}: ${s ? s.name : es.shift_id}</span>`;
    }

    const startD = es.start_date ? es.start_date.substring(0, 10) : '—';
    const endD = es.end_date ? es.end_date.substring(0, 10) : (window._lang === 'vi' ? 'Vô thời hạn' : 'Indefinite');

    return `
      <tr>
        <td><strong>${empName}</strong></td>
        <td><code>${empCode}</code></td>
        <td>${assignDetail}</td>
        <td>${startD}</td>
        <td>${endD}</td>
        <td>
          <button class="secondary-btn" style="color: var(--danger); border-color: var(--danger);" onclick="deleteEmployeeShift('${es.employee_id}', '${es.id}')">${t('btnDelete')}</button>
        </td>
      </tr>`;
  }).join('');

  // Update Filter Tabs
  let filterHtml = `<button class="ghost-btn ${selectedFilter === 'all' ? 'active' : ''}" onclick="filterShiftsList('all')">${t('filterAll')}</button>`;
  state.shifts.forEach(s => {
    filterHtml += `<button class="ghost-btn ${selectedFilter === s.id ? 'active' : ''}" onclick="filterShiftsList('${s.id}')">${s.name}</button>`;
  });
  state.rotationPatterns.forEach(rp => {
    filterHtml += `<button class="ghost-btn ${selectedFilter === rp.id ? 'active' : ''}" onclick="filterShiftsList('${rp.id}')">${rp.name}</button>`;
  });
  els.shiftFilterTabs.innerHTML = filterHtml;

  // Render Shift Swaps
  els.shiftSwapTableBody.innerHTML = state.shiftSwaps.map((sw) => {
    const reqEmp = state.employees.find(e => e.id === sw.requester_employee_id);
    const tarEmp = state.employees.find(e => e.id === sw.target_employee_id);
    const reqName = reqEmp ? `${reqEmp.full_name} (${reqEmp.employee_code})` : sw.requester_employee_id;
    const tarName = tarEmp ? `${tarEmp.full_name} (${tarEmp.employee_code})` : sw.target_employee_id;
    
    let statusClass = "waiting";
    let statusText = t('optStatusPending');
    if (sw.status === 'approved') {
      statusClass = "success";
      statusText = t('optStatusApproved');
    } else if (sw.status === 'rejected') {
      statusClass = "danger";
      statusText = t('optStatusRejected');
    }

    const actionHtml = sw.status === 'pending' ? `
      <button class="ghost-btn" style="color: var(--success); border-color: var(--success); padding: 4px 8px;" onclick="approveShiftSwap('${sw.id}')">${t('btnApprove')}</button>
      <button class="ghost-btn" style="color: var(--danger); border-color: var(--danger); padding: 4px 8px;" onclick="rejectShiftSwap('${sw.id}')">${t('btnReject')}</button>
    ` : '—';

    return `
      <tr>
        <td>
          <strong>${reqName}</strong><br/>
          <span style="font-size:0.85rem; color:var(--muted);">${sw.requested_date ? sw.requested_date.substring(0,10) : '—'}</span>
        </td>
        <td>
          <strong>${tarName}</strong><br/>
          <span style="font-size:0.85rem; color:var(--muted);">${sw.target_date ? sw.target_date.substring(0,10) : '—'}</span>
        </td>
        <td><span class="badge ${statusClass}">${statusText}</span></td>
        <td>${actionHtml}</td>
      </tr>`;
  }).join('');

  // Update employee select dropdowns
  const empOpts = state.employees.map((e) => `<option value="${e.id}">${e.full_name} (${e.employee_code})</option>`).join('');
  els.assignEmployeeId.innerHTML = empOpts;
  if (els.swapRequesterEmployeeId) els.swapRequesterEmployeeId.innerHTML = empOpts;
  if (els.swapTargetEmployeeId) els.swapTargetEmployeeId.innerHTML = empOpts;
  if (els.batchAssignEmployeesSelect) els.batchAssignEmployeesSelect.innerHTML = empOpts;

  // Update shift select dropdowns
  const shiftOpts = state.shifts.map((s) => `<option value="${s.id}">${s.name} (${s.start_time} - ${s.end_time})</option>`).join('');
  els.assignShiftId.innerHTML = shiftOpts;
  if (els.batchAssignShiftId) els.batchAssignShiftId.innerHTML = shiftOpts;

  // Update rotation select dropdowns
  const rotOpts = state.rotationPatterns.map((rp) => `<option value="${rp.id}">${rp.name}</option>`).join('');
  els.assignRotationPatternId.innerHTML = rotOpts;
  if (els.batchAssignRotationPatternId) els.batchAssignRotationPatternId.innerHTML = rotOpts;

  // Update departments dropdown for batch assign
  const depts = [...new Set(state.employees.map(e => e.department_id).filter(Boolean))];
  if (els.batchAssignDeptSelect) {
    els.batchAssignDeptSelect.innerHTML = depts.map(d => `<option value="${d}">${t('optDept' + d.toUpperCase()) || d}</option>`).join('');
  }

  // Default dates
  const todayStr = getLocalDateStr(new Date());
  if (!els.assignStartDate.value) els.assignStartDate.value = todayStr;
  if (els.batchAssignStartDate && !els.batchAssignStartDate.value) els.batchAssignStartDate.value = todayStr;
  if (els.swapRequesterDate && !els.swapRequesterDate.value) els.swapRequesterDate.value = todayStr;
  if (els.swapTargetDate && !els.swapTargetDate.value) els.swapTargetDate.value = todayStr;
}

window.filterShiftsList = function(id) {
  state.selectedShiftFilter = id;
  renderShifts();
};

window.reindexRotationSteps = reindexRotationSteps;

async function onSaveShift(event) {
  event.preventDefault();
  const payload = {
    name: els.shiftName.value,
    start_time: els.shiftStartTime.value,
    end_time: els.shiftEndTime.value,
    break_minutes: Number(els.shiftBreakMinutes.value || 60),
    late_grace_minutes: Number(els.shiftLateGrace.value || 15),
    early_grace_minutes: Number(els.shiftEarlyGrace.value || 15),
    max_working_minutes: 480,
    timezone: "Asia/Ho_Chi_Minh"
  };
  try {
    let response;
    if (state.editingShiftId) {
      response = await api(`/api/v1/shifts/${state.editingShiftId}`, { method: 'PUT', body: JSON.stringify(payload) });
    } else {
      response = await api('/api/v1/shifts', { method: 'POST', body: JSON.stringify(payload) });
    }
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Không thể lưu ca' : 'Failed to save shift');
    showToast(state.editingShiftId ? (window._lang === 'vi' ? 'Cập nhật ca thành công!' : 'Shift updated successfully!') : t('toastCreateShiftSuccess'), 'success');
    els.shiftForm.reset();
    els.shiftModal.classList.remove('active');
    state.editingShiftId = null;
    await loadShifts();
    renderShifts();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function onAssignShift(event) {
  event.preventDefault();
  const employeeId = els.assignEmployeeId.value;
  const type = els.assignType.value;
  
  const payload = {
    start_date: els.assignStartDate.value,
    end_date: els.assignEndDate.value ? els.assignEndDate.value : undefined
  };

  if (type === 'shift') {
    payload.shift_id = els.assignShiftId.value;
  } else {
    payload.rotation_pattern_id = els.assignRotationPatternId.value;
  }

  try {
    const response = await api(`/api/v1/employees/${employeeId}/shifts`, { method: 'POST', body: JSON.stringify(payload) });
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Không thể gán ca' : 'Failed to assign shift');
    showToast(t('toastAssignShiftSuccess'), 'success');
    els.assignShiftModal.classList.remove('active');
    await loadShifts();
    renderShifts();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function onBatchAssignShift(event) {
  event.preventDefault();
  const target = els.batchAssignTargetType.value;
  const type = els.batchAssignType.value;
  
  const payload = {
    start_date: els.batchAssignStartDate.value,
    end_date: els.batchAssignEndDate.value ? els.batchAssignEndDate.value : undefined
  };

  let employeeIds = [];
  if (target === 'all') {
    employeeIds = state.employees.map(e => e.id);
    if (!employeeIds.length) {
      showToast(window._lang === 'vi' ? 'Không có nhân viên nào trong hệ thống' : 'No employees in the system', 'error');
      return;
    }
  } else if (target === 'department') {
    const deptId = els.batchAssignDeptSelect.value;
    employeeIds = state.employees.filter(e => e.department_id === deptId).map(e => e.id);
    if (!employeeIds.length) {
      showToast(window._lang === 'vi' ? 'Không có nhân viên nào trong phòng ban này' : 'No employees in this department', 'error');
      return;
    }
  } else if (target === 'multiple') {
    const selectedOptions = Array.from(els.batchAssignEmployeesSelect.selectedOptions);
    employeeIds = selectedOptions.map(opt => opt.value);
    if (!employeeIds.length) {
      showToast(window._lang === 'vi' ? 'Vui lòng chọn ít nhất một nhân viên' : 'Please select at least one employee', 'error');
      return;
    }
  }

  payload.employee_ids = employeeIds;

  if (type === 'shift') {
    payload.shift_id = els.batchAssignShiftId.value;
  } else {
    payload.rotation_pattern_id = els.batchAssignRotationPatternId.value;
  }

  try {
    const response = await api('/api/v1/shifts/assign-batch', { method: 'POST', body: JSON.stringify(payload) });
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Gán ca hàng loạt thất bại' : 'Batch shift assignment failed');
    showToast(window._lang === 'vi' ? 'Gán ca hàng loạt thành công!' : 'Batch shift assignment successful!', 'success');
    els.batchAssignModal.classList.remove('active');
    await loadShifts();
    renderShifts();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function onCreateRotationPattern(event) {
  event.preventDefault();
  
  const stepRows = els.rotationStepsContainer.querySelectorAll('.rotation-step-row');
  const sequence = [];
  stepRows.forEach(row => {
    const shiftId = row.querySelector('.rotation-shift-select').value;
    const duration = Number(row.querySelector('.rotation-duration-input').value || 1);
    sequence.push({ shift_id: shiftId, duration: duration });
  });

  if (!sequence.length) {
    showToast(window._lang === 'vi' ? 'Vui lòng thêm ít nhất một ca vào chu kỳ' : 'Please add at least one shift to sequence', 'error');
    return;
  }

  const payload = {
    name: els.rotationName.value,
    pattern: sequence
  };

  try {
    let response;
    if (state.editingRotationId) {
      response = await api(`/api/v1/rotation-patterns/${state.editingRotationId}`, { method: 'PUT', body: JSON.stringify(payload) });
    } else {
      response = await api('/api/v1/rotation-patterns', { method: 'POST', body: JSON.stringify(payload) });
    }
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Tạo/Cập nhật chu kỳ xoay ca thất bại' : 'Failed to save rotation pattern');
    showToast(state.editingRotationId ? (window._lang === 'vi' ? 'Cập nhật chu kỳ xoay ca thành công!' : 'Rotation pattern updated successfully!') : (window._lang === 'vi' ? 'Tạo chu kỳ xoay ca thành công!' : 'Rotation pattern created successfully!'), 'success');
    els.rotationModal.classList.remove('active');
    state.editingRotationId = null;
    await loadShifts();
    renderShifts();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function onCreateShiftSwap(event) {
  event.preventDefault();
  
  const payload = {
    requester_employee_id: els.swapRequesterEmployeeId.value,
    requested_date: els.swapRequesterDate.value,
    target_employee_id: els.swapTargetEmployeeId.value,
    target_date: els.swapTargetDate.value
  };

  if (payload.requester_employee_id === payload.target_employee_id) {
    showToast(window._lang === 'vi' ? 'Không thể đổi ca với chính mình' : 'Cannot swap shift with yourself', 'error');
    return;
  }

  try {
    const response = await api('/api/v1/shift-swaps', { method: 'POST', body: JSON.stringify(payload) });
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Gửi yêu cầu đổi ca thất bại' : 'Failed to submit shift swap');
    showToast(window._lang === 'vi' ? 'Gửi yêu cầu đổi ca thành công!' : 'Shift swap requested successfully!', 'success');
    els.shiftSwapModal.classList.remove('active');
    await loadShifts();
    renderShifts();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function deleteShift(id) {
  if (!confirm(window._lang === 'vi' ? 'Bạn chắc chắn muốn xoá ca này?' : 'Are you sure you want to delete this shift?')) return;
  try {
    const response = await api(`/api/v1/shifts/${id}`, { method: 'DELETE' });
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Không thể xoá ca' : 'Failed to delete shift');
    showToast(t('toastDeleteShiftSuccess'), 'success');
    await loadShifts();
    renderShifts();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

window.deleteShift = deleteShift;

function editShift(id) {
  const shift = state.shifts.find(s => s.id === id);
  if (!shift) {
    showToast(window._lang === 'vi' ? 'Không tìm thấy ca làm việc' : 'Shift not found', 'error');
    return;
  }
  
  state.editingShiftId = id;
  els.shiftName.value = shift.name;
  els.shiftStartTime.value = shift.start_time ? shift.start_time.substring(0, 5) : '';
  els.shiftEndTime.value = shift.end_time ? shift.end_time.substring(0, 5) : '';
  els.shiftBreakMinutes.value = shift.break_minutes;
  els.shiftLateGrace.value = shift.late_grace_minutes;
  els.shiftEarlyGrace.value = shift.early_grace_minutes;

  const headerTitle = els.shiftModal.querySelector('.modal-header h3');
  if (headerTitle) headerTitle.textContent = t('modalEditShift');
  const submitBtn = els.shiftForm.querySelector('button[type="submit"]');
  if (submitBtn) submitBtn.textContent = t('btnSave') || 'Lưu';

  els.shiftModal.classList.add('active');
}
window.editShift = editShift;

function editRotationPattern(id) {
  const rp = state.rotationPatterns.find(r => r.id === id);
  if (!rp) {
    showToast(window._lang === 'vi' ? 'Không tìm thấy chu kỳ xoay ca' : 'Rotation pattern not found', 'error');
    return;
  }
  
  state.editingRotationId = id;
  els.rotationName.value = rp.name;

  let steps = [];
  try {
    const seqStr = rp.pattern_sequence || rp.pattern || "[]";
    steps = typeof seqStr === 'string' ? JSON.parse(seqStr) : seqStr;
  } catch (e) {
    console.error('Error parsing rotation pattern:', e);
  }

  els.rotationStepsContainer.innerHTML = '';
  if (Array.isArray(steps) && steps.length > 0) {
    steps.forEach((step, idx) => {
      const div = document.createElement('div');
      div.className = 'rotation-step-row';
      div.style = 'display: flex; gap: 10px; align-items: center; margin-bottom: 5px;';
      const options = state.shifts.map(s => `<option value="${s.id}" ${s.id === step.shift_id ? 'selected' : ''}>${s.name}</option>`).join('');
      div.innerHTML = `
        <span style="font-size: 0.9rem; font-weight: 500; min-width: 60px;">Bước ${idx + 1}:</span>
        <select class="form-input rotation-shift-select" style="flex-grow: 1; height: 38px;" required>
          ${options}
        </select>
        <input type="number" class="form-input rotation-duration-input" placeholder="Số ngày" min="1" value="${step.duration || 1}" style="width: 85px; height: 38px;" required />
        <button type="button" class="ghost-btn" style="color: var(--danger); border-color: var(--danger); padding: 4px 8px; height: 38px;" onclick="this.parentElement.remove(); reindexRotationSteps();">✕</button>
      `;
      els.rotationStepsContainer.appendChild(div);
    });
  } else {
    addRotationStep();
  }

  const headerTitle = els.rotationModal.querySelector('.modal-header h3');
  if (headerTitle) headerTitle.textContent = t('modalEditRotation');
  const submitBtn = els.rotationForm.querySelector('button[type="submit"]');
  if (submitBtn) submitBtn.textContent = t('btnSave') || 'Lưu';

  els.rotationModal.classList.add('active');
}
window.editRotationPattern = editRotationPattern;

window.deleteRotationPattern = async function(id) {
  if (!confirm(window._lang === 'vi' ? 'Bạn chắc chắn muốn xoá chu kỳ xoay ca này?' : 'Are you sure you want to delete this rotation pattern?')) return;
  try {
    const response = await api(`/api/v1/rotation-patterns/${id}`, { method: 'DELETE' });
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Không thể xoá chu kỳ' : 'Failed to delete rotation pattern');
    showToast(window._lang === 'vi' ? 'Xoá chu kỳ thành công' : 'Rotation pattern deleted successfully', 'success');
    await loadShifts();
    renderShifts();
  } catch (error) {
    showToast(error.message, 'error');
  }
};

window.deleteEmployeeShift = async function(employeeId, assignmentId) {
  if (!confirm(window._lang === 'vi' ? 'Bạn chắc chắn muốn huỷ gán ca cho nhân viên này?' : 'Are you sure you want to delete this shift assignment?')) return;
  try {
    const response = await api(`/api/v1/employee-shifts/${assignmentId}`, { method: 'DELETE' });
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Không thể huỷ gán ca' : 'Failed to delete shift assignment');
    showToast(window._lang === 'vi' ? 'Huỷ gán ca thành công' : 'Shift assignment deleted successfully', 'success');
    await loadShifts();
    renderShifts();
  } catch (error) {
    showToast(error.message, 'error');
  }
};

window.approveShiftSwap = async function(swapId) {
  try {
    const response = await api(`/api/v1/shift-swaps/${swapId}/approve`, { method: 'POST' });
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Duyệt đổi ca thất bại' : 'Failed to approve swap');
    showToast(window._lang === 'vi' ? 'Đã duyệt đổi ca thành công' : 'Shift swap approved successfully', 'success');
    await loadShifts();
    renderShifts();
  } catch (error) {
    showToast(error.message, 'error');
  }
};

window.rejectShiftSwap = async function(swapId) {
  const reason = prompt(window._lang === 'vi' ? 'Nhập lý do từ chối:' : 'Enter reason for rejection:');
  if (reason === null) return;
  try {
    const response = await api(`/api/v1/shift-swaps/${swapId}/reject`, {
      method: 'POST',
      body: JSON.stringify({ reason: reason })
    });
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Từ chối đổi ca thất bại' : 'Failed to reject swap');
    showToast(window._lang === 'vi' ? 'Đã từ chối đổi ca' : 'Shift swap rejected', 'success');
    await loadShifts();
    renderShifts();
  } catch (error) {
    showToast(error.message, 'error');
  }
};
async function onProcessAttendance() {
  const dateStr = els.attendanceFrom.value || getLocalDateStr(new Date());
  try {
    showToast(`${t('toastProcessingAttendance')} ${dateStr}...`, 'info');
    const response = await api('/api/v1/daily-attendance/process', {
      method: 'POST',
      body: JSON.stringify({ date: dateStr })
    });
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Tính công thất bại' : 'Failed to process attendance');
    showToast(`${t('toastProcessAttendanceSuccess')} ${dateStr}!`, 'success');
    await loadAttendance();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function readJsonResponse(response) {
  if (!response) return null;
  const text = await response.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch (error) {
    console.warn('Unable to parse JSON response:', error);
    return null;
  }
}

async function api(url, options = {}) {
  const headers = {
    ...(options.headers || {}),
    ...(state.token ? { Authorization: `Bearer ${state.token}` } : {})
  };
  if (!headers['Content-Type'] && !(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
  }
  
  let response;
  try {
    response = await fetch(url, { ...options, headers });
  } catch (netErr) {
    console.error('Network error calling API:', netErr);
    throw new Error(window._lang === 'vi' ? `Lỗi kết nối mạng: ${netErr.message || netErr}` : `Network connection error: ${netErr.message || netErr}`);
  }

  if (response.status === 401) {
    logout();
    throw new Error(window._lang === 'vi' ? 'Phiên làm việc đã hết hạn, vui lòng đăng nhập lại.' : 'Session expired, please login again.');
  }

  if (!response.ok) {
    let errMsg = `Yêu cầu thất bại (HTTP ${response.status})`;
    try {
      const errorText = await response.text();
      try {
        const errorPayload = JSON.parse(errorText);
        if (errorPayload && errorPayload.error) {
          errMsg = errorPayload.error;
        } else if (errorPayload && errorPayload.message) {
          errMsg = errorPayload.message;
        } else {
          errMsg = `${errMsg}: ${errorText}`;
        }
      } catch (jsonErr) {
        if (errorText) {
          errMsg = `${errMsg}: ${errorText}`;
        }
      }
    } catch (readErr) {
      console.error('Error reading response body:', readErr);
    }
    throw new Error(errMsg);
  }
  return response;
}

let todayChartInstance = null;
let weeklyChartInstance = null;

async function loadAuditLogs() {
  try {
    const response = await api('/api/v1/audit-logs?limit=50');
    const data = await response.json();
    state.auditLogs = Array.isArray(data) ? data : [];
    renderAuditLogs();
  } catch (err) {
    console.error('Failed to load audit logs:', err);
    state.auditLogs = [];
  }
}

function renderAuditLogs() {
  if (!els.auditTableBody) return;
  els.auditTableBody.innerHTML = (state.auditLogs || []).map(log => {
    return `
      <tr>
        <td><code>${new Date(log.created_at || Date.now()).toLocaleString(window._lang === 'vi' ? 'vi-VN' : 'en-US')}</code></td>
        <td><strong>${log.user_id || 'System'}</strong></td>
        <td><span class="badge" style="background: rgba(99, 102, 241, 0.15); color: #a5b4fc; border:1px solid rgba(99,102,241,0.3); font-weight:600; padding: 2px 6px;">${log.action}</span></td>
        <td><span class="muted">${log.object_type || '-'}</span></td>
        <td>${log.description || ''}</td>
        <td><code>${log.ip_address || '-'}</code></td>
      </tr>
    `;
  }).join('') || `<tr><td colspan="6" class="muted" style="text-align:center;">${window._lang === 'vi' ? 'Chưa ghi nhận hoạt động nào' : 'No activities recorded'}</td></tr>`;
}

async function loadMonthlyReport() {
  const monthVal = els.reportMonth.value;
  if (!monthVal) {
    showToast(t('toastSelectMonth'), 'error');
    return;
  }
  
  els.reportMessage.textContent = t('reportLoading');
  els.reportMessage.className = 'message info';
  
  try {
    const [year, month] = monthVal.split('-').map(Number);
    const numDays = new Date(year, month, 0).getDate();
    
    const fromDate = `${monthVal}-01`;
    const toDate = `${monthVal}-${String(numDays).padStart(2, '0')}`;
    
    const response = await api(`/api/v1/daily-attendance/report?from=${fromDate}&to=${toDate}`);
    const reports = await response.json();
    
    renderMonthlyReport(year, month, numDays, reports);
    els.reportMessage.textContent = '';
    els.reportMessage.className = 'message hidden';
  } catch (err) {
    els.reportMessage.textContent = err.message;
    els.reportMessage.className = 'message error';
    showToast(err.message, 'error');
  }
}

function renderMonthlyReport(year, month, numDays, reports) {
	if (!Array.isArray(reports)) reports = [];
  let headerHtml = `
    <tr>
      <th style="position: sticky; left: 0; background: var(--panel); z-index: 10;">${window._lang === 'vi' ? 'Mã NV' : 'Emp Code'}</th>
      <th style="position: sticky; left: 80px; background: var(--panel); z-index: 10; min-width: 150px;">${window._lang === 'vi' ? 'Họ tên' : 'Full Name'}</th>
  `;
  for (let d = 1; d <= numDays; d++) {
    headerHtml += `<th style="text-align:center; min-width: 40px; font-size:0.85rem;">${d}</th>`;
  }
  headerHtml += `
      <th style="text-align:center; min-width: 80px;">${window._lang === 'vi' ? 'Công (ngày)' : 'Workdays'}</th>
      <th style="text-align:center; min-width: 80px;">${t('thSumLate')}</th>
      <th style="text-align:center; min-width: 80px;">${window._lang === 'vi' ? 'Sớm (phút)' : 'Early (min)'}</th>
      <th style="text-align:center; min-width: 80px;">${t('thSumOT')}</th>
    </tr>
  `;
  els.reportTableHeader.innerHTML = headerHtml;

  const employeeMap = {};
  state.employees.forEach(emp => {
    employeeMap[emp.id] = {
      code: emp.employee_code,
      name: emp.full_name,
      days: {}
    };
  });

  reports.forEach(r => {
    const empId = r.employee_id;
    if (!employeeMap[empId]) {
      employeeMap[empId] = {
        code: r.employee_id,
        name: r.employee_id,
        days: {}
      };
    }
    const day = new Date(r.date).getDate();
    employeeMap[empId].days[day] = r;
  });

  const rows = Object.keys(employeeMap).map(empId => {
    const emp = employeeMap[empId];
    let rowHtml = `
      <tr>
        <td style="position: sticky; left: 0; background: var(--panel); z-index: 9;"><code>${emp.code}</code></td>
        <td style="position: sticky; left: 80px; background: var(--panel); z-index: 9;"><strong>${emp.name}</strong></td>
    `;
    
    let totalWorkingHours = 0;
    let totalLate = 0;
    let totalEarly = 0;
    let totalOvertime = 0;

    for (let d = 1; d <= numDays; d++) {
      const dayData = emp.days[d];
      if (dayData) {
        totalWorkingHours += dayData.working_hours || 0;
        totalLate += dayData.late_minutes || 0;
        totalEarly += dayData.early_minutes || 0;
        totalOvertime += dayData.overtime_minutes || 0;

        let symbol = '❌'; 
        let cellColor = 'rgba(239, 68, 68, 0.12)'; 
        let titleText = t('absent');
        
        if (dayData.attendance_status === 'present') {
          symbol = '✔️';
          cellColor = 'rgba(16, 185, 129, 0.15)'; 
          titleText = window._lang === 'vi' ? `Đủ công (${dayData.working_hours.toFixed(1)}h)` : `Present (${dayData.working_hours.toFixed(1)}h)`;
        } else if (dayData.attendance_status === 'late') {
          symbol = '⚠️';
          cellColor = 'rgba(245, 158, 11, 0.15)'; 
          titleText = window._lang === 'vi' ? `Đi muộn ${dayData.late_minutes} phút` : `Late ${dayData.late_minutes} mins`;
        } else if (dayData.attendance_status === 'early') {
          symbol = '⚠️';
          cellColor = 'rgba(245, 158, 11, 0.15)';
          titleText = window._lang === 'vi' ? `Về sớm ${dayData.early_minutes} phút` : `Early ${dayData.early_minutes} mins`;
        } else if (dayData.attendance_status === 'leave') {
          symbol = '🍀';
          cellColor = 'rgba(59, 130, 246, 0.15)'; 
          titleText = t('leave');
        }

        rowHtml += `<td style="text-align:center; background-color: ${cellColor};" title="${titleText}">${symbol}</td>`;
      } else {
        rowHtml += `<td style="text-align:center; background-color: rgba(255,255,255,0.02); color:#555;">-</td>`;
      }
    }

    const workedDays = totalWorkingHours / 8.0;

    rowHtml += `
        <td style="text-align:center; font-weight:600; color:var(--success);">${workedDays.toFixed(1)}</td>
        <td style="text-align:center; color:${totalLate > 0 ? 'var(--warning)' : 'var(--muted)'};">${totalLate}</td>
        <td style="text-align:center; color:${totalEarly > 0 ? 'var(--warning)' : 'var(--muted)'};">${totalEarly}</td>
        <td style="text-align:center; color:${totalOvertime > 0 ? '#38bdf8' : 'var(--muted)'}; font-weight:600;">+${totalOvertime}</td>
      </tr>
    `;
    return rowHtml;
  }).join('');

  els.reportTableBody.innerHTML = rows || `<tr><td colspan="${numDays + 6}" class="muted" style="text-align:center;">${t('reportNoData')}</td></tr>`;
}

function syncEmployeeToDevice(employeeId) {
  const employee = state.employees.find(e => e.id === employeeId);
  if (!employee) return;
  openSyncSingleEmployeeModal(employeeId, employee.employee_code, employee.full_name);
}

async function confirmFingerprintEnrollment(employeeId) {
  try {
    const response = await api(`/api/v1/employees/${employeeId}/device-mappings`);
    const mappings = await response.json();
    if (!mappings.length) {
      showToast(window._lang === 'vi' ? 'Nhân viên chưa được đồng bộ lên thiết bị.' : 'Employee not synced to device.', 'error');
      return;
    }
    
    const proceedWithMapping = (mapping) => {
      const confirmMsg = window._lang === 'vi' 
        ? `Xác nhận đã đăng ký vân tay trực tiếp trên máy cho ID ${mapping.device_user_id}?` 
        : `Confirm fingerprint has been enrolled directly on device for ID ${mapping.device_user_id}?`;
        
      customConfirm(confirmMsg, async () => {
        try {
          await api(`/api/v1/employees/${employeeId}/devices/${mapping.device_id}/fingerprint-confirm`, { method: 'POST', body: JSON.stringify({}) });
          showToast(window._lang === 'vi' ? 'Đã ghi nhận trạng thái đăng ký vân tay.' : 'Recorded fingerprint enrollment status.', 'success');
        } catch (err) {
          showToast(err.message, 'error');
        }
      });
    };

    if (mappings.length > 1) {
      const options = mappings.map((m, i) => ({
        value: i,
        label: window._lang === 'vi' ? `ID máy: ${m.device_user_id}` : `Device ID: ${m.device_user_id}`
      }));
      const title = window._lang === 'vi' ? 'Chọn Mapping' : 'Select Mapping';
      const msg = window._lang === 'vi' ? 'Chọn thiết bị đã đăng ký vân tay:' : 'Select device with enrolled fingerprint:';
      customPromptSelect(title, msg, options, (selectedIndex) => {
        const mapping = mappings[Number(selectedIndex)];
        if (mapping) proceedWithMapping(mapping);
      });
    } else {
      proceedWithMapping(mappings[0]);
    }
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function exportMonthlyReport() {
  const monthVal = els.reportMonth.value;
  if (!monthVal) {
    showToast(t('toastSelectMonth'), 'error');
    return;
  }
  try {
    const response = await api(`/api/v1/reports/attendance-excel?month=${encodeURIComponent(monthVal)}`);
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `BaoCaoChamCong_${monthVal}.csv`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
    showToast(t('toastExportSuccess'), 'success');
  } catch (error) {
    showToast(error.message, 'error');
  }
}

function updateCharts() {
  const ctxToday = document.getElementById('todayAttendanceChart');
  const ctxWeekly = document.getElementById('weeklyAttendanceChart');
  if (!ctxToday || !ctxWeekly) return;

  const totalEmployees = state.employees.length || 5; 
  const todayStr = getLocalDateStr(new Date());
  const todaySummary = state.attendanceSummary.filter(item => {
    const d = getLocalDateStr(item.date);
    return d === todayStr;
  });
  
  let present = todaySummary.filter(item => item.attendance_status === 'present').length;
  let partial = todaySummary.filter(item => item.attendance_status === 'late' || item.attendance_status === 'early').length;
  let leave = todaySummary.filter(item => item.attendance_status === 'leave').length;
  let absent = Math.max(0, totalEmployees - (present + partial + leave));

  if (present === 0 && partial === 0 && leave === 0 && state.attendance.length === 0) {
    present = 3;
    partial = 1;
    leave = 0;
    absent = 1;
  }

  const isLight = document.body.classList.contains('light-theme');
  const chartTextColor = isLight ? '#0f172a' : '#f8fafc';
  const chartMutedColor = isLight ? '#64748b' : '#94a3b8';
  const chartGridColor = isLight ? 'rgba(0, 0, 0, 0.08)' : 'rgba(255, 255, 255, 0.05)';
  const chartLineBorder = isLight ? '#2563eb' : '#6366f1';
  const chartLineBg = isLight ? 'rgba(37, 99, 235, 0.12)' : 'rgba(99, 102, 241, 0.15)';

  if (todayChartInstance) {
    todayChartInstance.destroy();
  }
  todayChartInstance = new Chart(ctxToday, {
    type: 'doughnut',
    data: {
      labels: window._lang === 'vi' ? ['Đủ công', 'Thiếu check / Trễ', 'Nghỉ phép', 'Vắng mặt'] : ['Present', 'Late / Partial Check', 'On Leave', 'Absent'],
      datasets: [{
        data: [present, partial, leave, absent],
        backgroundColor: ['#10b981', '#f59e0b', '#3b82f6', '#ef4444'],
        borderWidth: 0
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: {
          position: 'bottom',
          labels: { color: chartTextColor, font: { family: 'Outfit', size: 11 } }
        }
      }
    }
  });

  const last7Days = [];
  for (let i = 6; i >= 0; i--) {
    const d = new Date();
    d.setDate(d.getDate() - i);
    last7Days.push(getLocalDateStr(d));
  }

  const counts = last7Days.map(dateStr => {
    return state.attendance.filter(log => {
      const logDate = getLocalDateStr(log.check_time);
      return logDate === dateStr;
    }).length;
  });

  const hasData = counts.some(c => c > 0);
  const chartData = hasData ? counts : [5, 8, 12, 7, 9, 14, 11];

  if (weeklyChartInstance) {
    weeklyChartInstance.destroy();
  }
  weeklyChartInstance = new Chart(ctxWeekly, {
    type: 'line',
    data: {
      labels: last7Days.map(d => d.split('-').slice(1).reverse().join('/')), 
      datasets: [{
        label: window._lang === 'vi' ? 'Số lượt quét thẻ' : 'Number of card scans',
        data: chartData,
        borderColor: chartLineBorder,
        backgroundColor: chartLineBg,
        fill: true,
        tension: 0.4,
        borderWidth: 3
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      scales: {
        x: { grid: { color: chartGridColor }, ticks: { color: chartMutedColor, font: { family: 'Outfit' } } },
        y: { grid: { color: chartGridColor }, ticks: { color: chartMutedColor, font: { family: 'Outfit' } } }
      },
      plugins: {
        legend: { display: false }
      }
    }
  });
}

function showToast(message, type = 'info') {
  console.log('[toast]', type, message);
  const safeMessage = String(message || 'No message');
  const root = document.body || document.documentElement;
  const box = document.createElement('div');
  box.setAttribute('role', 'status');
  box.style.position = 'fixed';
  box.style.top = '20px';
  box.style.left = '50%';
  box.style.transform = 'translateX(-50%)';
  box.style.zIndex = '2147483647';
  box.style.minWidth = '320px';
  box.style.maxWidth = '560px';
  box.style.padding = '14px 16px';
  box.style.borderRadius = '12px';
  box.style.color = '#fff';
  box.style.fontSize = '14px';
  box.style.fontFamily = 'Arial, sans-serif';
  box.style.boxShadow = '0 20px 40px rgba(0,0,0,0.3)';
  box.style.background = type === 'success' ? '#10b981' : type === 'error' ? '#ef4444' : type === 'warning' ? '#f59e0b' : '#2563eb';
  box.style.border = '1px solid rgba(255,255,255,0.25)';
  box.textContent = safeMessage;
  root.appendChild(box);

  window.setTimeout(() => {
    box.remove();
  }, 3200);
}

function showConfirm(message, onConfirm, options = {}) {
  const title = options.title || (window._lang === 'vi' ? 'Xác nhận' : 'Confirm');
  const confirmText = options.confirmText || (window._lang === 'vi' ? 'Đồng ý' : 'OK');
  const cancelText = options.cancelText || (window._lang === 'vi' ? 'Hủy' : 'Cancel');
  const onCancel = typeof options.onCancel === 'function' ? options.onCancel : null;

  const root = document.body || document.documentElement;
  const overlay = document.createElement('div');
  overlay.style.position = 'fixed';
  overlay.style.inset = '0';
  overlay.style.zIndex = '2147483647';
  overlay.style.background = 'rgba(0,0,0,0.75)';
  overlay.style.display = 'flex';
  overlay.style.alignItems = 'center';
  overlay.style.justifyContent = 'center';
  overlay.style.padding = '20px';

  overlay.innerHTML = `
    <div style="width:min(420px,100%);background:#111827;border:1px solid rgba(255,255,255,0.18);border-radius:16px;padding:22px;color:#f9fafb;box-shadow:0 20px 45px rgba(0,0,0,0.4);">
      <div style="font-size:2rem;text-align:center;margin-bottom:10px;">⚠️</div>
      <h3 style="margin:0 0 10px;text-align:center;font-size:1.05rem;font-weight:600;">${title}</h3>
      <p style="margin:0 0 18px;text-align:center;line-height:1.5;color:#d1d5db;">${String(message || '')}</p>
      <div style="display:flex;justify-content:center;gap:10px;">
        <button type="button" id="confirmCancelBtn" style="min-width:100px;padding:10px 14px;border-radius:10px;border:1px solid #4b5563;background:#374151;color:#f9fafb;cursor:pointer;">${cancelText}</button>
        <button type="button" id="confirmOkBtn" style="min-width:100px;padding:10px 14px;border-radius:10px;border:1px solid #ef4444;background:#ef4444;color:white;cursor:pointer;">${confirmText}</button>
      </div>
    </div>
  `;

  root.appendChild(overlay);
  const okBtn = overlay.querySelector('#confirmOkBtn');
  const cancelBtn = overlay.querySelector('#confirmCancelBtn');

  okBtn.addEventListener('click', () => {
    overlay.remove();
    if (typeof onConfirm === 'function') onConfirm();
  });
  cancelBtn.addEventListener('click', () => {
    overlay.remove();
    if (typeof onCancel === 'function') onCancel();
  });
  overlay.addEventListener('click', (event) => {
    if (event.target === overlay) {
      overlay.remove();
      if (typeof onCancel === 'function') onCancel();
    }
  });
}

function customConfirm(message, onConfirm, options = {}) {
  showConfirm(message, onConfirm, options);
}

/**
 * Hiển thị hộp thoại chọn mapping tùy chỉnh (không dùng prompt mặc định).
 */
function customPromptSelect(title, message, options, onSelect, onCancel) {
  const modal = document.getElementById('selectMappingModal');
  const titleEl = document.getElementById('selectMappingModalTitle');
  const msgEl = document.getElementById('selectMappingModalMessage');
  const dropdown = document.getElementById('selectMappingDropdown');
  const submitBtn = document.getElementById('selectMappingModalSubmitBtn');
  const cancelBtn = document.getElementById('selectMappingModalCancelBtn');
  
  if (!modal || !dropdown || !submitBtn || !cancelBtn) {
    console.error('customPromptSelect: Modal elements missing from DOM');
    if (options.length > 0 && typeof onSelect === 'function') {
      onSelect(options[0].value); // Fallback chọn option đầu tiên
    }
    return;
  }
  
  titleEl.textContent = title;
  msgEl.textContent = message;
  dropdown.innerHTML = options.map(opt => `<option value="${opt.value}">${opt.label}</option>`).join('');
  
  const newSubmit = submitBtn.cloneNode(true);
  const newCancel = cancelBtn.cloneNode(true);
  
  newSubmit.textContent = window._lang === 'vi' ? 'Xác nhận' : 'Confirm';
  newCancel.textContent = window._lang === 'vi' ? 'Hủy' : 'Cancel';
  
  submitBtn.parentNode.replaceChild(newSubmit, submitBtn);
  cancelBtn.parentNode.replaceChild(newCancel, cancelBtn);
  
  const closeModal = () => closeModalElement(modal);
  
  newSubmit.addEventListener('click', () => {
    closeModal();
    if (typeof onSelect === 'function') onSelect(dropdown.value);
  });
  
  newCancel.addEventListener('click', () => {
    closeModal();
    if (typeof onCancel === 'function') onCancel();
  });
  
  const backdrop = modal.querySelector('.modal-backdrop');
  if (backdrop) {
    const newBackdrop = backdrop.cloneNode(true);
    backdrop.parentNode.replaceChild(newBackdrop, backdrop);
    newBackdrop.addEventListener('click', closeModal);
  }
  
  modal.classList.add('active');
  // Fallbacks in case CSS or rendering is blocked: force display and set body lock
  try {
    modal.style.display = 'flex';
    document.body.classList.add('modal-open');
    console.log('selectMapping modal opened (active class added)');
  } catch (e) {
    console.warn('failed to apply modal fallbacks', e);
  }
}


// ============================================================
// ESS & Request Management (Leave, Overtime, Correction)
// ============================================================

function toggleEssFormFields() {
  const type = els.essType.value;
  els.essLeaveFields.classList.toggle('hidden', type !== 'leave');
  els.essOvertimeFields.classList.toggle('hidden', type !== 'overtime');
  els.essCorrectionFields.classList.toggle('hidden', type !== 'correction');
}


function populateEssEmployees() {
  const select = document.getElementById('essEmployeeId') || (els && els.essEmployeeId);
  if (!select) return;
  const employees = state.employees || [];
  if (employees.length === 0) {
    select.innerHTML = `<option value="">${window._lang === 'vi' ? '-- Chưa có nhân viên --' : '-- No employees --'}</option>`;
    return;
  }
  select.innerHTML = employees.map(e => 
    `<option value="${e.id}">${e.full_name} (${e.employee_code})</option>`
  ).join('');
}

async function loadEssInitialData() {
  if (!state.employees || state.employees.length === 0) {
    await loadEmployees();
  }
  populateEssEmployees();

  // Default dates to today
  const today = getLocalDateStr(new Date());
  els.essLeaveStartDate.value = today;
  els.essLeaveEndDate.value = today;
  els.essOtDate.value = today;
  els.essCorrDate.value = today;

  await loadEssRequestsList();
}

async function loadEssRequestsList() {
  const filterType = els.essFilterType.value;
  const filterStatus = els.essFilterStatus.value;
  
  try {
    els.essMessage.textContent = '';
    let list = [];

    // Tải các loại đơn từ tương ứng dựa vào filter
    if (filterType === 'all' || filterType === 'leave') {
      const resp = await api('/api/v1/leave-requests');
      const leaves = await resp.json();
      if (Array.isArray(leaves)) {
        list = list.concat(leaves.map(item => ({ ...item, ess_type: 'leave' })));
      }
    }
    if (filterType === 'all' || filterType === 'overtime') {
      const resp = await api('/api/v1/overtime-requests');
      const ots = await resp.json();
      if (Array.isArray(ots)) {
        list = list.concat(ots.map(item => ({ ...item, ess_type: 'overtime' })));
      }
    }
    if (filterType === 'all' || filterType === 'correction') {
      const resp = await api('/api/v1/attendance-corrections');
      const corrs = await resp.json();
      if (Array.isArray(corrs)) {
        list = list.concat(corrs.map(item => ({ ...item, ess_type: 'correction' })));
      }
    }

    // Lọc theo trạng thái
    if (filterStatus && filterStatus !== 'all') {
      list = list.filter(item => item.status === filterStatus);
    }

    // Sắp xếp giảm dần theo thời gian tạo
    list.sort((a, b) => new Date(b.created_at || 0) - new Date(a.created_at || 0));
    state.essRequests = list;

    renderEssRequests();
  } catch (error) {
    console.error('Failed to load ESS requests:', error);
    els.essMessage.textContent = window._lang === 'vi' ? 'Không thể tải danh sách đơn từ' : 'Failed to load requests';
    els.essMessage.className = 'message error';
  }
}

function renderEssRequests() {
  if (state.essRequests.length === 0) {
    els.essTableBody.innerHTML = `<tr><td colspan="4" class="muted" style="text-align:center;">${window._lang === 'vi' ? 'Không có đơn từ nào' : 'No requests found'}</td></tr>`;
    return;
  }

  els.essTableBody.innerHTML = state.essRequests.map(item => {
    const emp = state.employees.find(e => e.id === item.employee_id);
    const empName = emp ? `${emp.full_name} (${emp.employee_code})` : `NV ID: ${item.employee_id}`;

    let details = '';
    let typeBadge = '';
    if (item.ess_type === 'leave') {
      typeBadge = `<span class="badge" style="background-color:#3b82f6;">Nghỉ phép</span>`;
      details = `<strong>${item.leave_type.toUpperCase()}</strong>: Từ ${getLocalDateStr(item.start_date)} đến ${getLocalDateStr(item.end_date)}<br/><span class="muted">${item.reason}</span>`;
    } else if (item.ess_type === 'overtime') {
      typeBadge = `<span class="badge" style="background-color:#8b5cf6;">Làm thêm giờ</span>`;
      details = `Ngày: ${getLocalDateStr(item.date)} (${item.start_time} - ${item.end_time})<br/><span class="muted">${item.reason}</span>`;
    } else {
      typeBadge = `<span class="badge" style="background-color:#ec4899;">Sửa công</span>`;
      const checkLabel = item.check_type === 'in' ? 'Check-In' : 'Check-Out';
      details = `Ngày: ${getLocalDateStr(item.date)} | ${checkLabel} lúc <strong>${item.corrected_time}</strong><br/><span class="muted">${item.reason}</span>`;
    }

    let statusClass = 'pending';
    if (item.status === 'approved') statusClass = 'online';
    if (item.status === 'rejected') statusClass = 'offline';
    
    let statusText = '';
    if (item.status === 'approved') {
      statusText = t('optStatusApproved');
    } else if (item.status === 'rejected') {
      statusText = t('optStatusRejected');
    } else {
      statusText = t('optStatusPending');
    }

    let actionsHtml = '-';
    if (item.status === 'pending') {
      actionsHtml = `
        <div style="display:flex; gap:4px;">
          <button class="primary-btn" style="padding:4px 8px; font-size:12px; background-color:#10b981; border:none;" data-action="approve-ess" data-type="${item.ess_type}" data-id="${item.id}">${t('btnApprove')}</button>
          <button class="secondary-btn" style="padding:4px 8px; font-size:12px;" data-action="reject-ess" data-type="${item.ess_type}" data-id="${item.id}">${t('btnReject')}</button>
        </div>
      `;
    }

    return `
      <tr>
        <td>
          <div style="font-weight:600;">${empName}</div>
          <div style="margin-top:4px;">${typeBadge}</div>
        </td>
        <td>${details}</td>
        <td><span class="badge ${statusClass}">${statusText}</span></td>
        <td>${actionsHtml}</td>
      </tr>
    `;
  }).join('');
}

async function onSubmitEssRequest(event) {
  event.preventDefault();
  const type = els.essType.value;
  const employeeId = els.essEmployeeId.value;
  const reason = els.essReason.value.trim();

  let url = '';
  let payload = { employee_id: employeeId, reason: reason };

  if (type === 'leave') {
    url = '/api/v1/leave-requests';
    payload.leave_type = els.essLeaveType.value;
    payload.start_date = els.essLeaveStartDate.value;
    payload.end_date = els.essLeaveEndDate.value;
  } else if (type === 'overtime') {
    url = '/api/v1/overtime-requests';
    payload.date = els.essOtDate.value;
    payload.start_time = els.essOtStartTime.value;
    payload.end_time = els.essOtEndTime.value;
  } else if (type === 'correction') {
    url = '/api/v1/attendance-corrections';
    payload.date = els.essCorrDate.value;
    payload.check_type = els.essCorrCheckType.value;
    payload.corrected_time = els.essCorrTime.value;
  }

  try {
    const response = await api(url, {
      method: 'POST',
      body: JSON.stringify(payload)
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'Failed to submit request');
    
    showToast(window._lang === 'vi' ? 'Đã gửi yêu cầu đăng ký thành công!' : 'Request submitted successfully!', 'success');
    els.essReason.value = '';
    els.essModal.classList.remove('active');
    await loadEssRequestsList();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function handleEssApproval(action, type, id) {
  let url = '';
  if (type === 'leave') {
    url = `/api/v1/leave-requests/${id}/${action}`;
  } else if (type === 'overtime') {
    url = `/api/v1/overtime-requests/${id}/${action}`;
  } else if (type === 'correction') {
    url = `/api/v1/attendance-corrections/${id}/${action}`;
  }

  try {
    const response = await api(url, { method: 'POST', body: JSON.stringify({}) });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'Action failed');

    const actionText = action === 'approve' 
      ? (window._lang === 'vi' ? 'phê duyệt' : 'approved') 
      : (window._lang === 'vi' ? 'từ chối' : 'rejected');
    showToast(window._lang === 'vi' ? `Đã ${actionText} yêu cầu thành công!` : `Request ${actionText} successfully!`, 'success');
    
    await loadEssRequestsList();
    loadAll(); // Reload report / stats ngầm
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function loadSystemConfig() {
  try {
    const response = await api('/api/v1/system/config');
    const cfg = await response.json();
    
    const dbText = `${cfg.db_name || 'attendance'} trên host ${cfg.db_host || 'localhost'}`;
    const dbEl = document.getElementById('settingsDbInfo');
    if (dbEl) dbEl.textContent = dbText;
    
    const cronText = cfg.cron_spec || '0 */15 * * * * (15 phút/lần)';
    const cronEl = document.getElementById('settingsCronInfo');
    if (cronEl) cronEl.textContent = cronText;
    
    let ldapText = window._lang === 'vi' ? '🔴 Đang Tắt' : '🔴 Disabled';
    if (cfg.ldap_enabled) {
      ldapText = window._lang === 'vi' 
        ? `🟢 Đang Bật (${cfg.ldap_domain} @ ${cfg.ldap_url})`
        : `🟢 Enabled (${cfg.ldap_domain} @ ${cfg.ldap_url})`;
    }
    const ldapEl = document.getElementById('settingsLdapInfo');
    if (ldapEl) ldapEl.textContent = ldapText;
    
    const portEl = document.getElementById('settingsPortInfo');
    if (portEl) portEl.textContent = cfg.http_port || '8085';
  } catch (err) {
    console.error('Failed to load system config:', err);
    const cronEl = document.getElementById('settingsCronInfo');
    if (cronEl) cronEl.textContent = 'Error';
    const ldapEl = document.getElementById('settingsLdapInfo');
    if (ldapEl) ldapEl.textContent = 'Error';
    const portEl = document.getElementById('settingsPortInfo');
    if (portEl) portEl.textContent = 'Error';
  }
}


// ============================================================
// Các hàm đồng bộ hóa và đăng ký vân tay mới
// ============================================================

/**
 * Đẩy 1 nhân viên xuống TẤT CẢ thiết bị ADMS đang online.
 */
async function pushEmployeeToAllDevices(employeeId, empName) {
  const name = empName || (state.employees.find(e => e.id === employeeId) || {}).full_name || employeeId;
  try {
    const response = await api(`/api/v1/employees/${employeeId}/push-to-all-devices`, { method: 'POST' });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'Failed');
    const msg = window._lang === 'vi'
      ? `📤 "Đã đưa lệnh cập nhật nhân viên "${name}" vào queue ${data.success_count} thiết bị!`
      : `📤 Queued employee "${name}" update to ${data.success_count} device(s)!`;
    showToast(msg, 'success');
    if (data.errors && data.errors.length > 0) {
      console.warn('Push to devices - some errors:', data.errors);
      const errMsg = window._lang === 'vi'
        ? `Lưu ý: ${data.errors.join(', ')}`
        : `Warnings: ${data.errors.join(', ')}`;
      showToast(errMsg, 'warning');
    }
  } catch (err) {
    showToast(err.message, 'error');
  }
}

async function clearEmployeeEnrollData(employeeId) {
  const emp = state.employees.find(e => e.id === employeeId);
  const empName = emp ? emp.full_name : employeeId;
  try {
    showConfirm(
      window._lang === 'vi'
        ? `Xác nhận xóa toàn bộ dữ liệu vân tay của nhân viên "${empName}" trên TẤT CẢ thiết bị ADMS? Thao tác không thể hoàn tác.`
        : `Confirm clearing all fingerprint data for employee "${empName}" from ALL ADMS devices? This action cannot be undone.`,
      async () => {
        try {
          showToast(window._lang === 'vi' ? `Đang gửi lệnh xóa dữ liệu vân tay...` : `Sending clear fingerprint data command...`, 'info');
          const response = await api(`/api/v1/employees/${employeeId}/clear-enroll-data`, { method: 'POST' });
          const data = await response.json();
          if (!response.ok) throw new Error(data.error || 'Failed');
          showToast(window._lang === 'vi' ? `Đã gửi lệnh xóa dữ liệu vân tay của "${empName}" xuống các thiết bị ADMS.` : `Clear fingerprint data command sent for "${empName}".`, 'success');
          await loadAll();
        } catch (error) {
          showToast(error?.message || 'Failed to clear enroll data', 'error');
        }
      }
    );
  } catch (error) {
    showToast(error?.message || 'Failed to clear enroll data', 'error');
  }
}

/**
 * Mở modal đẩy TẤT CẢ nhân viên xuống 1 thiết bị ADMS.
 */
// ADMS commands are retained by the server until the terminal polls them, so
// a stale status indicator must not hide a usable ADMS terminal.
function getOnlineADMSDevice() {
	const toolbarSelect = document.getElementById('stopBatchEnrollDeviceSelect');
	const selectedDevice = toolbarSelect?.value
	  ? state.devices.find(d => d.id === toolbarSelect.value)
	  : null;
	if (selectedDevice) return selectedDevice;
  return state.devices.find(d => d.adms_enabled && d.status === 'online')
    || state.devices.find(d => d.adms_enabled)
	  || state.devices.find(d => !d.adms_enabled)
    || null;
}

function sendMonthlyReportsByEmail() {
  const monthVal = els.reportMonth.value;
  if (!monthVal) {
    showToast(t('toastSelectMonth'), 'error');
    return;
  }
  const employees = Array.isArray(state.employees) ? state.employees : [];
  const withEmail = employees.filter(employee => String(employee?.email || '').trim()).length;
  const withoutEmail = Math.max(0, employees.length - withEmail);
  const message = window._lang === 'vi'
    ? `Gửi báo cáo tháng ${monthVal} riêng cho từng nhân viên có email?\n\nCó email: ${withEmail}; chưa có email: ${withoutEmail}.\nDữ liệu lấy từ bảng công đã tính và không tự tính công.`
    : `Send ${monthVal} reports individually to employees with email?\n\nWith email: ${withEmail}; without email: ${withoutEmail}.\nData comes from calculated attendance and will not calculate attendance automatically.`;
  customConfirm(message, () => executeMonthlyReportsByEmail(monthVal), {
    title: window._lang === 'vi' ? 'Gửi báo cáo tháng qua Gmail' : 'Send monthly reports by Gmail',
    confirmText: window._lang === 'vi' ? 'Gửi báo cáo' : 'Send reports'
  });
}

async function executeMonthlyReportsByEmail(monthVal) {
  const button = els.sendMonthlyReportBtn;
  const originalText = button ? button.textContent : '';
  if (button) { button.disabled = true; button.textContent = window._lang === 'vi' ? 'Đang gửi...' : 'Sending...'; }
  try {
    const response = await api('/api/v1/reports/monthly-email', {
      method: 'POST',
      body: JSON.stringify({ month: monthVal, force_resend: false })
    });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || (window._lang === 'vi' ? 'Gửi báo cáo thất bại' : 'Failed to send reports'));
    const message = window._lang === 'vi'
      ? `Đã gửi ${data.sent || 0} email; thiếu email: ${data.skipped_no_email || 0}; email sai: ${data.skipped_invalid_email || 0}; lỗi: ${data.failed || 0}.`
      : `Sent ${data.sent || 0} emails; missing email: ${data.skipped_no_email || 0}; invalid email: ${data.skipped_invalid_email || 0}; failed: ${data.failed || 0}.`;
    showToast(message, (data.failed || 0) > 0 ? 'warning' : 'success');
  } catch (error) {
    showToast(error.message, 'error');
  } finally {
    if (button) { button.disabled = false; button.textContent = originalText; }
  }
}

async function runPushAllToOnlineDevice() {
  const device = getOnlineADMSDevice();
  if (!device) {
    showToast('Thất bại: không có thiết bị ADMS đang online.', 'error');
    return;
  }
  showToast(`Đang đẩy toàn bộ nhân viên xuống máy "${device.name}"...`, 'info');
  try {
    const response = await api(`/api/v1/devices/${device.id}/sync-employees`, { method: 'POST' });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Không thể đẩy nhân viên');
    showToast(`Thành công: đã gửi ${data?.record_count ?? 0} nhân viên tới máy "${device.name}".`, 'success');
    await loadAll();
  } catch (error) {
    showToast(`Thất bại: ${error.message}`, 'error');
  }
}

async function runBatchEnrollForPendingEmployees() {
  console.log('runBatchEnrollForPendingEmployees invoked');
  const device = getOnlineADMSDevice();
  if (!device) {
    showToast('Thất bại: không có thiết bị ADMS đang online.', 'error');
    return;
  }
  const employeeIds = state.employees
    .filter(e => (e.status || 'active') === 'active' && !e.fingerprint_enrolled)
    .map(e => e.id);
  if (employeeIds.length === 0) {
    showToast('Thành công: không có nhân viên nào cần quét vân tay.', 'success');
    return;
  }
  showToast(`Đang gửi quét vân tay cho ${employeeIds.length} nhân viên trên máy "${device.name}"...`, 'info');
  try {
    const response = await api('/api/v1/employees/batch-enroll', {
      method: 'POST',
      body: JSON.stringify({ employee_ids: employeeIds, device_id: device.id })
    });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Không thể gửi yêu cầu quét vân tay');
    const queued = data?.enqueued ?? 0;
    const requested = data?.total_requested ?? employeeIds.length;
    showToast(`Thành công: đã gửi ${queued}/${requested} yêu cầu quét vân tay tới máy "${device.name}".`, 'success');
  } catch (error) {
    showToast(`Thất bại: ${error.message}`, 'error');
  }
}

function openPushAllToDeviceModal() {
  const modal = document.getElementById('pushAllToDeviceModal');
  const select = document.getElementById('pushAllDeviceSelect');
  const statusMsg = document.getElementById('pushAllStatusMsg');
  if (!modal || !select) return;

  const admsDevices = state.devices.filter(Boolean);
  if (admsDevices.length === 0) {
    showToast(window._lang === 'vi' ? 'Không có thiết bị ADMS nào!' : 'No ADMS devices found!', 'warning');
    return;
  }
  select.innerHTML = admsDevices.map(d => {
    const online = d.status === 'online' ? ' ✅ Online' : ' ⚠ Offline';
    return `<option value="${d.id}">${d.name} (${d.adms_enabled ? 'ADMS' : 'SDK'})${online}</option>`;
  }).join('');
  statusMsg.style.display = 'none';
  modal.classList.add('active');
}

async function confirmPushAllToDevice() {
  const deviceId = document.getElementById('pushAllDeviceSelect').value;
  const statusMsg = document.getElementById('pushAllStatusMsg');
  const btn = document.getElementById('confirmPushAllBtn');
  const modal = document.getElementById('pushAllToDeviceModal');

  if (!deviceId) return;
  const origText = btn.innerHTML;
  btn.disabled = true;
  btn.innerHTML = '⏳ Đang xử lý...';

  try {
    const response = await api(`/api/v1/devices/${deviceId}/sync-employees`, { method: 'POST' });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'Failed');
    statusMsg.className = 'message success';
    statusMsg.textContent = window._lang === 'vi'
      ? `✅ Đã đưa ${data.record_count || '?'} nhân viên vào hàng đợi máy!`
      : `✅ Queued ${data.record_count || '?'} employees to device!`;
    statusMsg.style.display = 'block';
    setTimeout(() => modal.classList.remove('active'), 2000);
  } catch (err) {
    statusMsg.className = 'message error';
    statusMsg.textContent = err.message;
    statusMsg.style.display = 'block';
  } finally {
    btn.disabled = false;
    btn.innerHTML = origText;
  }
}

/**
 * Mở Wizard quét vân tay hàng loạt.
 */
function populateStopBatchDeviceSelect() {
  const select = document.getElementById('stopBatchEnrollDeviceSelect');
  if (!select) return;
  const previousDeviceId = select.value;
  const enrollDevices = (state.devices || []).filter(Boolean);
  // Kept as an alias below while this toolbar control is shared by the
  // enrollment and ADMS-cancel actions.
  const admsDevices = enrollDevices;
  if (enrollDevices.length === 0) {
    select.innerHTML = '<option value="">-- Chưa có thiết bị ADMS --</option>';
    return;
  }
  select.innerHTML = ['<option value="">-- Chọn thiết bị --</option>'].concat(admsDevices.map(d => {
    const online = d.status === 'online' ? ' ✅' : ' ⚠';
    const label = d.name || d.ip_address || d.id;
    return `<option value="${d.id}">${online} ${label} (${d.adms_enabled ? 'ADMS' : 'SDK'})</option>`;
  })).join('');
  if (previousDeviceId && admsDevices.some(d => d.id === previousDeviceId)) {
    select.value = previousDeviceId;
  }
}

function openBatchEnrollWizard() {
  console.log('openBatchEnrollWizard invoked');
  const modal = document.getElementById('batchEnrollModal');
  const deviceSelect = document.getElementById('batchEnrollDeviceSelect');
  const empList = document.getElementById('batchEnrollEmployeeList');
  const statusMsg = document.getElementById('batchEnrollStatusMsg');
  if (!modal || !deviceSelect || !empList) {
    console.error('openBatchEnrollWizard: missing DOM elements', { modal: !!modal, deviceSelect: !!deviceSelect, empList: !!empList });
    showToast(window._lang === 'vi' ? 'Lỗi: giao diện quét hàng loạt chưa sẵn sàng. Vui lòng tải lại trang.' : 'Error: batch enroll UI not ready. Please reload the page.', 'error');
    return;
  }

  const enrollDevices = state.devices.filter(Boolean);
  if (enrollDevices.length === 0) {
    showToast(window._lang === 'vi' ? 'Không có thiết bị ADMS nào!' : 'No ADMS devices!', 'warning');
    return;
  }
  deviceSelect.innerHTML = enrollDevices.map(d => {
    const online = d.status === 'online' ? ' ✅' : ' ⚠';
    return `<option value="${d.id}">${online} ${d.name} (${d.adms_enabled ? 'ADMS' : 'SDK'})</option>`;
  }).join('');

  // Hiển thị tất cả nhân viên active
  const activeEmployees = state.employees.filter(e =>
    (e.status || 'active') === 'active' && !e.fingerprint_enrolled
  );
  empList.innerHTML = activeEmployees.map(emp => {
    const hasFP = emp.fingerprint_enrolled;
    const fpLabel = hasFP ? ' 🖐' : ' ⚠ chưa có VT';
    return `
      <label style="display: flex; align-items: center; gap: 10px; padding: 6px; border-radius: 6px; cursor: pointer; background: ${hasFP ? 'rgba(16,185,129,0.05)' : 'rgba(239,68,68,0.05)'}; border: 1px solid var(--border);">
        <input type="checkbox" value="${emp.id}" ${!hasFP ? 'checked' : ''} style="width:16px; height:16px;" />
        <div>
          <strong style="font-size:0.9rem;">${emp.full_name}</strong>
          <span class="muted" style="font-size:0.8rem; margin-left:6px;">${emp.employee_code}${fpLabel}</span>
        </div>
      </label>
    `;
  }).join('');

  statusMsg.style.display = 'none';
  document.getElementById('batchEnrollSelectAll').checked = activeEmployees.length > 0;
  document.getElementById('batchEnrollSelectAll').disabled = activeEmployees.length === 0;
  empList.querySelectorAll('input[type="checkbox"]').forEach(cb => { cb.checked = true; });
  modal.classList.add('active');
}

async function stopBatchEnroll(deviceSelectId = 'stopBatchEnrollDeviceSelect') {
  console.log('stopBatchEnroll invoked');
  const deviceSelect = document.getElementById(deviceSelectId);
  const statusMsg = document.getElementById('batchEnrollStatusMsg');
  const btn = document.getElementById('stopBatchEnrollBtn');
  if (!deviceSelect || !deviceSelect.value) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị trước khi dừng.' : 'Please select a device before stopping.', 'warning');
    return;
  }
  const device = state.devices.find(d => d.id === deviceSelect.value);
  const original = btn.innerHTML;
  btn.disabled = true;
  btn.innerHTML = '⏳ Đang dừng...';
  try {
    // This endpoint cancels either queued ADMS commands or the active SDK
    // batch context, depending on the selected device type.
    const response = await api(`/api/v1/devices/${deviceSelect.value}/cancel-pending-commands`, { method: 'POST' });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Failed');
    statusMsg.className = 'message warning';
    statusMsg.textContent = window._lang === 'vi'
      ? `🛑 Đã hủy ${data?.cancelled ?? 0} lệnh đang chờ trên thiết bị.`
      : `🛑 Cancelled ${data?.cancelled ?? 0} pending commands on the device.`;
    statusMsg.style.display = 'block';
    showToast(window._lang === 'vi' ? 'Đã dừng các lệnh quét đang chờ. Lưu ý: vân tay chỉ được lưu sau khi máy gửi xác nhận.' : 'Stopped pending enrollment commands. Note: templates are saved only after device confirmation.', 'warning');
  } catch (err) {
    statusMsg.className = 'message error';
    statusMsg.textContent = err.message;
    statusMsg.style.display = 'block';
  } finally {
    btn.disabled = false;
    btn.innerHTML = original;
  }
}

async function confirmBatchEnroll() {
  const deviceId = document.getElementById('batchEnrollDeviceSelect').value;
  const checkedBoxes = document.querySelectorAll('#batchEnrollEmployeeList input[type="checkbox"]:checked');
  const employeeIds = Array.from(checkedBoxes).map(cb => cb.value);
  const statusMsg = document.getElementById('batchEnrollStatusMsg');
  const btn = document.getElementById('confirmBatchEnrollBtn');
  const modal = document.getElementById('batchEnrollModal');

  if (!deviceId || employeeIds.length === 0) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị và ít nhất 1 nhân viên!' : 'Please select device and at least 1 employee!', 'warning');
    return;
  }

  const origText = btn.innerHTML;
  btn.disabled = true;
  btn.innerHTML = '⏳ Đang gửi lệnh...';

  try {
    const response = await api('/api/v1/employees/batch-enroll', {
      method: 'POST',
      body: JSON.stringify({ employee_ids: employeeIds, device_id: deviceId })
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'Failed');

    statusMsg.className = 'message success';
    statusMsg.textContent = window._lang === 'vi'
      ? `✅ ${data.message || 'Đã đưa lệnh vào hàng đợi!'}`
      : `✅ ${data.message || 'Commands queued!'}`;
    statusMsg.style.display = 'block';

    if (data.errors && data.errors.length > 0) {
      console.warn('Batch enroll errors:', data.errors);
    }
    setTimeout(() => modal.classList.remove('active'), 2500);
  } catch (err) {
    statusMsg.className = 'message error';
    statusMsg.textContent = err.message;
    statusMsg.style.display = 'block';
  } finally {
    btn.disabled = false;
    btn.innerHTML = origText;
  }
}

/**
 * Mở modal kéo nhân viên từ máy về web.
 */
function openPullFromDeviceModal() {
  const modal = document.getElementById('pullFromDeviceModal');
  const select = document.getElementById('pullFromDeviceSelect');
  const statusMsg = document.getElementById('pullFromDeviceStatusMsg');
  if (!modal || !select) return;

  if (state.devices.length === 0) {
    showToast(window._lang === 'vi' ? 'Không có thiết bị nào!' : 'No devices found!', 'warning');
    return;
  }
  select.innerHTML = state.devices.map(d => {
    const typeLabel = d.adms_enabled ? 'ADMS' : 'SDK';
    const online = d.status === 'online' ? '✅' : '⚠';
    return `<option value="${d.id}">${online} ${d.name} (${typeLabel})</option>`;
  }).join('');
  statusMsg.style.display = 'none';
  modal.classList.add('active');
}

async function confirmPullFromDevice() {
  const deviceId = document.getElementById('pullFromDeviceSelect').value;
  const statusMsg = document.getElementById('pullFromDeviceStatusMsg');
  const btn = document.getElementById('confirmPullFromDeviceBtn');
  if (!deviceId) {
    statusMsg.className = 'message error';
    statusMsg.textContent = window._lang === 'vi' ? '❌ Vui lòng chọn thiết bị nguồn.' : '❌ Please select a source device.';
    statusMsg.style.display = 'block';
    return;
  }
  const device = state.devices.find(d => d.id === deviceId);
  const deviceName = device?.name || (window._lang === 'vi' ? 'thiết bị đã chọn' : 'the selected device');
  const origText = btn.innerHTML;
  btn.disabled = true;
  statusMsg.className = 'message';
  statusMsg.textContent = window._lang === 'vi'
    ? `⏳ Đang kết nối và kéo nhân viên từ máy "${deviceName}". Vui lòng chờ...`
    : `⏳ Connecting and pulling employees from "${deviceName}". Please wait...`;
  statusMsg.style.display = 'block';
  btn.innerHTML = '⏳ Đang kéo dữ liệu...';

  try {
    const response = await api(`/api/v1/devices/${deviceId}/pull-employees`, { method: 'POST' });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to pull employees');

    const imported = Number(data?.imported || 0);
    const existing = Number(data?.existing || 0);
    const errors = Array.isArray(data?.errors) ? data.errors.filter(Boolean) : [];
    statusMsg.className = errors.length ? 'message warning' : 'message success';
    statusMsg.textContent = window._lang === 'vi'
      ? `${errors.length ? '⚠️' : '✅'} Kéo từ "${deviceName}" hoàn tất. Mới: ${imported}; đã có: ${existing}.${errors.length ? ` Lỗi: ${errors.join(' | ')}` : ''}`
      : `${errors.length ? '⚠️' : '✅'} Pull from "${deviceName}" completed. New: ${imported}; existing: ${existing}.${errors.length ? ` Errors: ${errors.join(' | ')}` : ''}`;
    statusMsg.style.display = 'block';
    showToast(window._lang === 'vi'
      ? `${errors.length ? 'Kéo dữ liệu hoàn tất nhưng có lỗi.' : 'Kéo nhân viên thành công.'} Mới: ${imported}, đã có: ${existing}.`
      : `${errors.length ? 'Pull completed with errors.' : 'Employees pulled successfully.'} New: ${imported}, existing: ${existing}.`, errors.length ? 'warning' : 'success');

    // Load lại danh sách nhân viên
    await loadEmployees();
    renderEmployees();
  } catch (err) {
    statusMsg.className = 'message error';
    statusMsg.textContent = window._lang === 'vi'
      ? `❌ Kéo nhân viên từ "${deviceName}" thất bại: ${err?.message || 'Không xác định được lỗi.'}`
      : `❌ Failed to pull employees from "${deviceName}": ${err?.message || 'Unknown error.'}`;
    statusMsg.style.display = 'block';
    showToast(statusMsg.textContent, 'error');
  } finally {
    btn.disabled = false;
    btn.innerHTML = origText;
  }
}

/**
 * Toggle hiển thị dropdown hành động trên dòng.
 */
function toggleRowDropdown(event, employeeId) {
  event.stopPropagation();
  // Đóng tất cả dropdown khác trước
  document.querySelectorAll('.row-dropdown-content').forEach(d => {
    if (d.id !== `dropdown-${employeeId}`) d.classList.remove('active', 'drop-up');
  });
  const content = document.getElementById(`dropdown-${employeeId}`);
  if (content) {
    content.classList.toggle('active');
    content.classList.remove('drop-up');
    if (content.classList.contains('active')) {
      requestAnimationFrame(() => {
        const rect = content.getBoundingClientRect();
        const triggerRect = content.previousElementSibling?.getBoundingClientRect();
        if (rect.bottom > window.innerHeight - 12 && triggerRect && triggerRect.top > rect.height + 12) {
          content.classList.add('drop-up');
        }
      });
    }
  }
}

/**
 * Mở modal chọn máy chấm công để đồng bộ nhân viên và quét vân tay.
        </div>
      </label>
    `;
  }).join('');

  statusMsg.style.display = 'none';
  document.getElementById('batchEnrollSelectAll').checked = activeEmployees.length > 0;
  document.getElementById('batchEnrollSelectAll').disabled = activeEmployees.length === 0;
  empList.querySelectorAll('input[type="checkbox"]').forEach(cb => { cb.checked = true; });
  modal.classList.add('active');
}

async function stopBatchEnroll(deviceSelectId = 'stopBatchEnrollDeviceSelect') {
  console.log('stopBatchEnroll invoked');
  const deviceSelect = document.getElementById(deviceSelectId);
  const statusMsg = document.getElementById('batchEnrollStatusMsg');
  const btn = document.getElementById('stopBatchEnrollBtn');
  if (!deviceSelect || !deviceSelect.value) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị trước khi dừng.' : 'Please select a device before stopping.', 'warning');
    return;
  }
  const device = state.devices.find(d => d.id === deviceSelect.value);
  const original = btn.innerHTML;
  btn.disabled = true;
  btn.innerHTML = '⏳ Đang dừng...';
  try {
    // This endpoint cancels either queued ADMS commands or the active SDK
    // batch context, depending on the selected device type.
    const response = await api(`/api/v1/devices/${deviceSelect.value}/cancel-pending-commands`, { method: 'POST' });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Failed');
    statusMsg.className = 'message warning';
    statusMsg.textContent = window._lang === 'vi'
      ? `🛑 Đã hủy ${data?.cancelled ?? 0} lệnh đang chờ trên thiết bị.`
      : `🛑 Cancelled ${data?.cancelled ?? 0} pending commands on the device.`;
    statusMsg.style.display = 'block';
    showToast(window._lang === 'vi' ? 'Đã dừng các lệnh quét đang chờ. Lưu ý: vân tay chỉ được lưu sau khi máy gửi xác nhận.' : 'Stopped pending enrollment commands. Note: templates are saved only after device confirmation.', 'warning');
  } catch (err) {
    statusMsg.className = 'message error';
    statusMsg.textContent = err.message;
    statusMsg.style.display = 'block';
  } finally {
    btn.disabled = false;
    btn.innerHTML = original;
  }
}

async function confirmBatchEnroll() {
  const deviceId = document.getElementById('batchEnrollDeviceSelect').value;
  const checkedBoxes = document.querySelectorAll('#batchEnrollEmployeeList input[type="checkbox"]:checked');
  const employeeIds = Array.from(checkedBoxes).map(cb => cb.value);
  const statusMsg = document.getElementById('batchEnrollStatusMsg');
  const btn = document.getElementById('confirmBatchEnrollBtn');
  const modal = document.getElementById('batchEnrollModal');

  if (!deviceId || employeeIds.length === 0) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị và ít nhất 1 nhân viên!' : 'Please select device and at least 1 employee!', 'warning');
    return;
  }

  const origText = btn.innerHTML;
  btn.disabled = true;
  btn.innerHTML = '⏳ Đang gửi lệnh...';

  try {
    const response = await api('/api/v1/employees/batch-enroll', {
      method: 'POST',
      body: JSON.stringify({ employee_ids: employeeIds, device_id: deviceId })
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'Failed');

    statusMsg.className = 'message success';
    statusMsg.textContent = window._lang === 'vi'
      ? `✅ ${data.message || 'Đã đưa lệnh vào hàng đợi!'}`
      : `✅ ${data.message || 'Commands queued!'}`;
    statusMsg.style.display = 'block';

    if (data.errors && data.errors.length > 0) {
      console.warn('Batch enroll errors:', data.errors);
    }
    setTimeout(() => modal.classList.remove('active'), 2500);
  } catch (err) {
    statusMsg.className = 'message error';
    statusMsg.textContent = err.message;
    statusMsg.style.display = 'block';
  } finally {
    btn.disabled = false;
    btn.innerHTML = origText;
  }
}

/**
 * Mở modal kéo nhân viên từ máy về web.
 */
function openPullFromDeviceModal() {
  const modal = document.getElementById('pullFromDeviceModal');
  const select = document.getElementById('pullFromDeviceSelect');
  const statusMsg = document.getElementById('pullFromDeviceStatusMsg');
  if (!modal || !select) return;

  if (state.devices.length === 0) {
    showToast(window._lang === 'vi' ? 'Không có thiết bị nào!' : 'No devices found!', 'warning');
    return;
  }
  select.innerHTML = state.devices.map(d => {
    const typeLabel = d.adms_enabled ? 'ADMS' : 'SDK';
    const online = d.status === 'online' ? '✅' : '⚠';
    return `<option value="${d.id}">${online} ${d.name} (${typeLabel})</option>`;
  }).join('');
  statusMsg.style.display = 'none';
  modal.classList.add('active');
}

async function confirmPullFromDevice() {
  const deviceId = document.getElementById('pullFromDeviceSelect').value;
  const statusMsg = document.getElementById('pullFromDeviceStatusMsg');
  const btn = document.getElementById('confirmPullFromDeviceBtn');
  if (!deviceId) {
    statusMsg.className = 'message error';
    statusMsg.textContent = window._lang === 'vi' ? '❌ Vui lòng chọn thiết bị nguồn.' : '❌ Please select a source device.';
    statusMsg.style.display = 'block';
    return;
  }
  const device = state.devices.find(d => d.id === deviceId);
  const deviceName = device?.name || (window._lang === 'vi' ? 'thiết bị đã chọn' : 'the selected device');
  const origText = btn.innerHTML;
  btn.disabled = true;
  statusMsg.className = 'message';
  statusMsg.textContent = window._lang === 'vi'
    ? `⏳ Đang kết nối và kéo nhân viên từ máy "${deviceName}". Vui lòng chờ...`
    : `⏳ Connecting and pulling employees from "${deviceName}". Please wait...`;
  statusMsg.style.display = 'block';
  btn.innerHTML = '⏳ Đang kéo dữ liệu...';

  try {
    const response = await api(`/api/v1/devices/${deviceId}/pull-employees`, { method: 'POST' });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to pull employees');

    const imported = Number(data?.imported || 0);
    const existing = Number(data?.existing || 0);
    const errors = Array.isArray(data?.errors) ? data.errors.filter(Boolean) : [];
    statusMsg.className = errors.length ? 'message warning' : 'message success';
    statusMsg.textContent = window._lang === 'vi'
      ? `${errors.length ? '⚠️' : '✅'} Kéo từ "${deviceName}" hoàn tất. Mới: ${imported}; đã có: ${existing}.${errors.length ? ` Lỗi: ${errors.join(' | ')}` : ''}`
      : `${errors.length ? '⚠️' : '✅'} Pull from "${deviceName}" completed. New: ${imported}; existing: ${existing}.${errors.length ? ` Errors: ${errors.join(' | ')}` : ''}`;
    statusMsg.style.display = 'block';
    showToast(window._lang === 'vi'
      ? `${errors.length ? 'Kéo dữ liệu hoàn tất nhưng có lỗi.' : 'Kéo nhân viên thành công.'} Mới: ${imported}, đã có: ${existing}.`
      : `${errors.length ? 'Pull completed with errors.' : 'Employees pulled successfully.'} New: ${imported}, existing: ${existing}.`, errors.length ? 'warning' : 'success');

    // Load lại danh sách nhân viên
    await loadEmployees();
    renderEmployees();
  } catch (err) {
    statusMsg.className = 'message error';
    statusMsg.textContent = window._lang === 'vi'
      ? `❌ Kéo nhân viên từ "${deviceName}" thất bại: ${err?.message || 'Không xác định được lỗi.'}`
      : `❌ Failed to pull employees from "${deviceName}": ${err?.message || 'Unknown error.'}`;
    statusMsg.style.display = 'block';
    showToast(statusMsg.textContent, 'error');
  } finally {
    btn.disabled = false;
    btn.innerHTML = origText;
  }
}

/**
 * Toggle hiển thị dropdown hành động trên dòng.
 */
function toggleRowDropdown(event, employeeId) {
  event.stopPropagation();
  // Đóng tất cả dropdown khác trước
  document.querySelectorAll('.row-dropdown-content').forEach(d => {
    if (d.id !== `dropdown-${employeeId}`) d.classList.remove('active', 'drop-up');
  });
  const content = document.getElementById(`dropdown-${employeeId}`);
  if (content) {
    content.classList.toggle('active');
    content.classList.remove('drop-up');
    if (content.classList.contains('active')) {
      requestAnimationFrame(() => {
        const rect = content.getBoundingClientRect();
        const triggerRect = content.previousElementSibling?.getBoundingClientRect();
        if (rect.bottom > window.innerHeight - 12 && triggerRect && triggerRect.top > rect.height + 12) {
          content.classList.add('drop-up');
        }
      });
    }
  }
}

/**
 * Mở modal chọn máy chấm công để đồng bộ nhân viên và quét vân tay.
 */
function openSyncSingleEmployeeModal(employeeId, employeeCode, employeeName) {
  if (!els.syncSingleEmployeeModal) return;
  els.syncSingleEmployeeEmpId.value = employeeId;
  els.syncSingleEmployeeEmpName.textContent = employeeName;
  els.syncSingleEmployeeDeviceUserId.value = employeeCode; // PIN mặc định là employeeCode

  const enrollDevices = state.devices.filter(Boolean);
  if (enrollDevices.length === 0) {
    showToast(window._lang === 'vi' ? 'Không có thiết bị ADMS nào!' : 'No ADMS devices found!', 'warning');
    return;
  }

  els.syncSingleEmployeeDeviceSelect.innerHTML = enrollDevices.map(d => {
    const online = d.status === 'online' ? ' ✅ Online' : ' ⚠ Offline';
    return `<option value="${d.id}">${d.name} (${d.ip_address || '-'})${online}</option>`;
  }).join('');

  els.syncSingleEmployeeStatusMsg.style.display = 'none';
  els.syncSingleEmployeeModal.classList.add('active');
}

/**
 * Xác nhận đồng bộ nhân viên và kích hoạt quét vân tay ngay lập tức.
 */
async function confirmSyncSingleEmployee() {
  const employeeId = els.syncSingleEmployeeEmpId.value;
  const deviceId = els.syncSingleEmployeeDeviceSelect.value;
  const deviceUserId = els.syncSingleEmployeeDeviceUserId.value.trim();
  const statusMsg = els.syncSingleEmployeeStatusMsg;
  const btn = els.confirmSyncSingleEmployeeBtn;

  if (!employeeId || !deviceId || !deviceUserId) {
    showToast(window._lang === 'vi' ? 'Vui lòng điền đầy đủ thông tin!' : 'Please fill all information!', 'warning');
    return;
  }

  const origText = btn.innerHTML;
  btn.disabled = true;
  btn.innerHTML = '⏳ Đang xử lý...';

  try {
    const response = await api(`/api/v1/employees/${employeeId}/devices/${deviceId}/sync`, {
      method: 'POST',
      body: JSON.stringify({ device_user_id: deviceUserId })
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'Failed');

    statusMsg.className = 'message success';
    statusMsg.textContent = window._lang === 'vi'
      ? `✅ Đã đồng bộ thông tin và yêu cầu quét vân tay xuống máy!`
      : `✅ Synced user and triggered remote enroll on device!`;
    statusMsg.style.display = 'block';

    showToast(window._lang === 'vi'
      ? 'Đã gửi yêu cầu quét tới máy; web sẽ chỉ hiện “Đã có” sau khi máy gửi dữ liệu vân tay xác nhận.'
      : 'Enrollment request queued; the web status will change only after the device sends fingerprint data.', 'info');
    
    // Tự động mở modal vân tay sau 1.5 giây
    setTimeout(() => {
      els.syncSingleEmployeeModal.classList.remove('active');
      openFingerprintModal(employeeId);
      setTimeout(() => {
        if (els.fpInstructionBox) els.fpInstructionBox.style.display = 'block';
      }, 500);
    }, 1500);

  } catch (err) {
    statusMsg.className = 'message error';
    statusMsg.textContent = err.message;
    statusMsg.style.display = 'block';
    showToast(err.message, 'error');
  } finally {
    btn.disabled = false;
    btn.innerHTML = origText;
  }
}

function deleteEmployeePrompt(employeeId) {
  console.log('[debug] deleteEmployeePrompt called', employeeId);
  showToast(window._lang === 'vi' ? 'Đang mở hộp xác nhận xoá...' : 'Opening delete confirmation...', 'info');
  customConfirm(
    window._lang === 'vi' ? 'Bạn có chắc chắn muốn xoá nhân viên này?' : 'Are you sure you want to delete this employee?',
    () => {
      showToast(window._lang === 'vi' ? 'Đã xác nhận xoá nhân viên' : 'Delete confirmed', 'warning');
      deleteEmployee(employeeId);
    }
  );
}



// ============================================================
// FACE RECOGNITION MODULE (face-api.js & Webcam)
// ============================================================

async function loadFaceApiModels() {
  if (state.faceModelsLoaded) return true;
  
  if (typeof faceapi === 'undefined') {
    showToast(window._lang === 'vi' ? 'Đang tải thư viện AI...' : 'Loading AI library...', 'info');
    try {
      await new Promise((resolve, reject) => {
        const script = document.createElement('script');
        script.src = '/face-api.min.js';
        script.onload = resolve;
        script.onerror = reject;
        document.head.appendChild(script);
      });
    } catch (e) {
      console.error('Failed to load face-api.min.js script:', e);
      showToast(window._lang === 'vi' ? 'Không thể tải thư viện AI khuôn mặt' : 'Failed to load face-api library', 'error');
      return false;
    }
  }

  if (typeof faceapi !== 'undefined' && faceapi.tf) {
    try {
      if (faceapi.tf.wasm && faceapi.tf.wasm.setWasmPaths) {
        faceapi.tf.wasm.setWasmPaths('/');
      }
      if (faceapi.tf.setBackend) {
        await faceapi.tf.setBackend('webgl').catch(() => faceapi.tf.setBackend('cpu'));
      }
      if (faceapi.tf.ready) {
        await faceapi.tf.ready();
      }
    } catch (backendErr) {
      console.warn('[FaceAPI] Backend init warning:', backendErr);
    }
  }

  try {
    showToast(window._lang === 'vi' ? 'Đang nạp mô hình AI khuôn mặt...' : 'Loading AI face models...', 'info');
    const MODEL_URL = '/models';
    await faceapi.nets.tinyFaceDetector.loadFromUri(MODEL_URL);
    await faceapi.nets.ssdMobilenetv1.loadFromUri(MODEL_URL);
    await faceapi.nets.faceLandmark68Net.loadFromUri(MODEL_URL);
    await faceapi.nets.faceRecognitionNet.loadFromUri(MODEL_URL);
    state.faceModelsLoaded = true;
    console.log('[FaceAPI] AI models loaded successfully from /models');
    showToast(window._lang === 'vi' ? '✅ Mô hình AI khuôn mặt đã sẵn sàng!' : '✅ Face AI models ready!', 'success');
    return true;
  } catch (err) {
    console.warn('[FaceAPI] Local models load failed, trying CDN fallback...', err);
    try {
      const CDN_URL = 'https://cdn.jsdelivr.net/npm/@vladmandic/face-api/model/';
      await faceapi.nets.tinyFaceDetector.loadFromUri(CDN_URL);
      await faceapi.nets.ssdMobilenetv1.loadFromUri(CDN_URL);
      await faceapi.nets.faceLandmark68Net.loadFromUri(CDN_URL);
      await faceapi.nets.faceRecognitionNet.loadFromUri(CDN_URL);
      state.faceModelsLoaded = true;
      showToast(window._lang === 'vi' ? '✅ Mô hình AI khuôn mặt đã sẵn sàng!' : '✅ Face AI models ready!', 'success');
      return true;
    } catch (cdnErr) {
      console.error('All face model loading attempts failed:', cdnErr);
      showToast(window._lang === 'vi' ? 'Không thể tải mô hình AI khuôn mặt' : 'Failed to load face AI models', 'error');
      return false;
    }
  }
}

async function loadRegisteredFaces() {
  try {
    if (!state.employees || state.employees.length === 0) {
      await loadEmployees();
    }
    if (!state.employeeMap) {
      state.employeeMap = {};
      (state.employees || []).forEach(e => { state.employeeMap[e.id] = e; });
    }
    const res = await api('/api/v1/faces');
    if (!res.ok) return;
    const faces = await res.json();
    state.registeredFaces = faces || [];
    
    const countEl = document.getElementById('registeredFacesCount');
    if (countEl) {
      countEl.textContent = window._lang === 'vi' 
        ? `Đã nạp ${state.registeredFaces.length} khuôn mặt` 
        : `Loaded ${state.registeredFaces.length} faces`;
    }

    if (state.registeredFaces.length === 0) {
      state.faceMatcher = null;
      return;
    }

    const labeledDescriptors = [];
    for (const f of state.registeredFaces) {
      try {
        const floatArr = new Float32Array(JSON.parse(f.face_descriptor));
        labeledDescriptors.push(new faceapi.LabeledFaceDescriptors(f.employee_id, [floatArr]));
      } catch (e) {
        console.error('Invalid face descriptor format:', e);
      }
    }

    if (labeledDescriptors.length > 0) {
      state.faceMatcher = new faceapi.FaceMatcher(labeledDescriptors, 0.68);
    }
  } catch (err) {
    console.error('Failed to load registered faces:', err);
  }
}

async function startCameraAttendance() {
  state.lastRecognizedTime = {}; // Reset chống trùng cho phiên camera mới
  const loaded = await loadFaceApiModels();
  if (!loaded) return;

  await loadRegisteredFaces();

  const video = document.getElementById('faceVideo');
  const overlay = document.getElementById('faceOverlay');
  const wrapper = document.getElementById('faceCameraWrapper');
  const laser = document.getElementById('scannerLaser');
  const statusPill = document.getElementById('cameraStatusPill');
  const startBtn = document.getElementById('startCameraButton');
  const stopBtn = document.getElementById('stopCameraButton');

  try {
    state.cameraStream = await navigator.mediaDevices.getUserMedia({
      video: { width: { ideal: 640 }, height: { ideal: 480 }, facingMode: 'user' }
    });
    if (video) {
      video.srcObject = state.cameraStream;
      await video.play();
    }

    if (wrapper) wrapper.classList.add('active');
    if (laser) laser.style.display = 'block';
    if (statusPill) {
      statusPill.className = 'status-pill online';
      statusPill.textContent = window._lang === 'vi' ? '● Camera đang quét' : '● Camera Scanning';
    }
    if (startBtn) startBtn.disabled = true;
    if (stopBtn) stopBtn.disabled = false;

    stopCameraAttendanceLoop();
    state.faceDetectTimer = setInterval(async () => {
      if (!video || video.paused || video.ended || state.isProcessingFaceScan) return;
      state.isProcessingFaceScan = true;
      try {

      const displaySize = { width: video.videoWidth || 640, height: video.videoHeight || 480 };
      if (overlay) faceapi.matchDimensions(overlay, displaySize);

      const detectorOptions = new faceapi.TinyFaceDetectorOptions({ inputSize: 160, scoreThreshold: 0.25 });
      let detections = await faceapi.detectAllFaces(video, detectorOptions)
        .withFaceLandmarks()
        .withFaceDescriptors();

      if (detections.length === 0 && faceapi.nets.ssdMobilenetv1.isLoaded) {
        detections = await faceapi.detectAllFaces(video, new faceapi.SsdMobilenetv1Options({ minConfidence: 0.4 }))
          .withFaceLandmarks()
          .withFaceDescriptors();
      }

      const resizedDetections = faceapi.resizeResults(detections, displaySize);
      if (overlay) {
        const ctx = overlay.getContext('2d');
        ctx.clearRect(0, 0, overlay.width, overlay.height);

        if (state.faceMatcher && resizedDetections.length > 0) {
          resizedDetections.forEach((detection) => {
            const match = state.faceMatcher.findBestMatch(detection.descriptor);
            const box = detection.detection.box;
            
            let label = window._lang === 'vi' ? 'Chưa đăng ký' : 'Unregistered';
            let color = '#EF4444';

            if (match.label !== 'unknown') {
              let emp = (state.employeeMap && state.employeeMap[match.label]) || (state.employees && state.employees.find(e => e.id === match.label));
              const empName = emp ? emp.full_name : (window._lang === 'vi' ? 'Nhân viên' : 'Employee');
              const empCode = emp ? emp.employee_code : '';

              const lastTime = state.lastRecognizedTime[match.label] || 0;
              const now = Date.now();
              const COOLDOWN_MS = 5 * 60 * 1000; // 5 phút chống trùng lặp điểm danh liên tục

              if (now - lastTime < COOLDOWN_MS) {
                // Đã điểm danh thành công trước đó
                label = `✅ ${empName} ${empCode ? '(' + empCode + ')' : ''} - ${window._lang === 'vi' ? 'Đã điểm danh' : 'Checked In'}`;
                color = '#06B6D4'; // Cyan ngọc nhã nhặn
              } else {
                // Lần đầu điểm danh hoặc hết thời gian chờ
                state.lastRecognizedTime[match.label] = now;
                label = `⚡ ${empName} ${empCode ? '(' + empCode + ')' : ''} (${Math.round((1 - match.distance) * 100)}%)`;
                color = '#10B981';
                console.log('[FaceAttendance] New check-in for:', match.label, empName);
                submitFaceAttendance(match.label);
              }
            }

            const drawBox = new faceapi.draw.DrawBox(box, { label, boxColor: color, lineWidth: 2 });
            drawBox.draw(overlay);
          });
        } else {
          faceapi.draw.drawDetections(overlay, resizedDetections);
        }
      }
      } finally {
        state.isProcessingFaceScan = false;
      }
    }, 30);

  } catch (err) {
    showToast(window._lang === 'vi' ? 'Không thể truy cập Camera: ' + err.message : 'Camera error: ' + err.message, 'error');
  }
}

function stopCameraAttendanceLoop() {
  if (state.faceDetectTimer) {
    clearInterval(state.faceDetectTimer);
    state.faceDetectTimer = null;
  }
}

function stopCameraAttendance() {
  state.lastRecognizedTime = {}; // Xóa chống trùng khi tắt camera
  stopCameraAttendanceLoop();
  stopCameraAttendanceLoop();
  if (state.cameraStream) {
    state.cameraStream.getTracks().forEach(track => track.stop());
    state.cameraStream = null;
  }

  const video = document.getElementById('faceVideo');
  const overlay = document.getElementById('faceOverlay');
  const wrapper = document.getElementById('faceCameraWrapper');
  const laser = document.getElementById('scannerLaser');
  const statusPill = document.getElementById('cameraStatusPill');
  const startBtn = document.getElementById('startCameraButton');
  const stopBtn = document.getElementById('stopCameraButton');

  if (video) video.srcObject = null;
  if (overlay) {
    const ctx = overlay.getContext('2d');
    ctx.clearRect(0, 0, overlay.width, overlay.height);
  }
  if (wrapper) wrapper.classList.remove('active');
  if (laser) laser.style.display = 'none';
  if (statusPill) {
    statusPill.className = 'status-pill offline';
    statusPill.textContent = window._lang === 'vi' ? '● Camera đã tắt' : '● Camera Offline';
  }
  if (startBtn) startBtn.disabled = false;
  if (stopBtn) stopBtn.disabled = true;
}

async function submitFaceAttendance(employeeId) {
  try {
    const res = await api('/api/v1/attendance/face-check', {
      method: 'POST',
      body: JSON.stringify({ employee_id: employeeId })
    });
    if (!res.ok) {
      const errData = await res.json();
      throw new Error(errData.error || 'Failed');
    }
    let emp = (state.employeeMap && state.employeeMap[employeeId]) || (state.employees && state.employees.find(e => e.id === employeeId));
    if (!emp) {
      await loadEmployees();
      emp = (state.employeeMap && state.employeeMap[employeeId]) || (state.employees && state.employees.find(e => e.id === employeeId));
    }
    const checkData = {
      employee_id: employeeId,
      employee_code: emp ? emp.employee_code : 'NV',
      full_name: emp ? emp.full_name : (window._lang === 'vi' ? 'Nhân viên' : 'Employee'),
      avatar_url: emp ? emp.avatar_url : '',
      department_name: emp ? (emp.job_title || emp.department_id || 'Nhân viên') : 'Nhân viên',
      check_time: new Date().toLocaleTimeString(window._lang === 'vi' ? 'vi-VN' : 'en-US'),
      check_type: 'IN (Face)',
      device_name: 'Camera Laptop'
    };
    showFaceAttendanceToast(checkData);
    // SSE event attendance_logged will append log row cleanly
    updateLastScanCard(checkData);
    showToast((window._lang === 'vi' ? '✅ Điểm danh thành công: ' : '✅ Attendance recorded: ') + checkData.full_name, 'success');
  } catch (err) {
    console.error('Submit face attendance error:', err);
    showToast(window._lang === 'vi' ? 'Lỗi điểm danh: ' + err.message : 'Attendance error: ' + err.message, 'error');
  }
}

async function openRegisterFaceModal(employeeId) {
  const emp = state.employees.find(e => e.id === employeeId);
  if (!emp) return;

  const modal = document.getElementById('registerFaceModal');
  const empNameEl = document.getElementById('registerFaceEmpName');
  const empIdEl = document.getElementById('registerFaceEmpId');
  const statusMsg = document.getElementById('registerFaceStatusMsg');
  const startBtn = document.getElementById('startRegisterFaceCamBtn');
  const captureBtn = document.getElementById('captureFaceBtn');

  if (empNameEl) empNameEl.textContent = `${emp.full_name} (${emp.employee_code})`;
  if (empIdEl) empIdEl.value = employeeId;
  if (statusMsg) {
    statusMsg.className = 'message';
    statusMsg.textContent = window._lang === 'vi' ? 'Bấm "Bật Camera" để bắt đầu quét' : 'Click "Start Camera" to begin';
  }
  if (startBtn) startBtn.disabled = false;
  if (captureBtn) captureBtn.disabled = true;

  if (modal) modal.classList.add('active');
  document.body.classList.add('modal-open');
}

async function startRegisterFaceCamera() {
  const loaded = await loadFaceApiModels();
  if (!loaded) return;

  const video = document.getElementById('registerFaceVideo');
  const overlay = document.getElementById('registerFaceOverlay');
  const statusMsg = document.getElementById('registerFaceStatusMsg');
  const wrapper = document.getElementById('registerFaceCameraWrapper');
  const startBtn = document.getElementById('startRegisterFaceCamBtn');
  const captureBtn = document.getElementById('captureFaceBtn');

  try {
    state.registerCameraStream = await navigator.mediaDevices.getUserMedia({
      video: { width: { ideal: 640 }, height: { ideal: 480 }, facingMode: 'user' }
    });
    if (video) {
      video.srcObject = state.registerCameraStream;
      await video.play();
    }

    if (wrapper) wrapper.classList.add('active');
    if (startBtn) startBtn.disabled = true;

    if (state.registerFaceDetectTimer) clearInterval(state.registerFaceDetectTimer);

    state.registerFaceDetectTimer = setInterval(async () => {
      if (!video || video.paused || video.ended || state.isProcessingRegisterScan) return;
      state.isProcessingRegisterScan = true;
      try {

      const displaySize = { width: video.videoWidth || 640, height: video.videoHeight || 480 };
      if (overlay) faceapi.matchDimensions(overlay, displaySize);

      const detectorOptions = new faceapi.TinyFaceDetectorOptions({ inputSize: 160, scoreThreshold: 0.25 });
      const detection = await faceapi.detectSingleFace(video, detectorOptions)
        .withFaceLandmarks()
        .withFaceDescriptor();

      if (overlay) {
        const ctx = overlay.getContext('2d');
        ctx.clearRect(0, 0, overlay.width, overlay.height);

        if (detection) {
          const resized = faceapi.resizeResults(detection, displaySize);
          faceapi.draw.drawDetections(overlay, resized);
          faceapi.draw.drawFaceLandmarks(overlay, resized);

          if (statusMsg) {
            statusMsg.className = 'message success';
            statusMsg.textContent = window._lang === 'vi' ? '✅ Đã phát hiện khuôn mặt! Sẵn sàng chụp.' : '✅ Face detected! Ready to capture.';
          }
          if (captureBtn) captureBtn.disabled = false;
          state.currentDetectedDescriptor = Array.from(detection.descriptor);
        } else {
          if (statusMsg) {
            statusMsg.className = 'message warning';
            statusMsg.textContent = window._lang === 'vi' ? '⚠️ Vui lòng nhìn thẳng vào camera...' : '⚠️ Please look straight into camera...';
          }
          if (captureBtn) captureBtn.disabled = true;
          state.currentDetectedDescriptor = null;
        }
      }
    } finally {
        state.isProcessingRegisterScan = false;
      }
    }, 30);

  } catch (err) {
    if (statusMsg) {
      statusMsg.className = 'message error';
      statusMsg.textContent = err.message;
    }
  }
}

function stopRegisterFaceCamera() {
  if (state.registerFaceDetectTimer) {
    clearInterval(state.registerFaceDetectTimer);
    state.registerFaceDetectTimer = null;
  }
  if (state.registerCameraStream) {
    state.registerCameraStream.getTracks().forEach(track => track.stop());
    state.registerCameraStream = null;
  }
  const video = document.getElementById('registerFaceVideo');
  const overlay = document.getElementById('registerFaceOverlay');
  const wrapper = document.getElementById('registerFaceCameraWrapper');
  if (video) video.srcObject = null;
  if (overlay) {
    const ctx = overlay.getContext('2d');
    ctx.clearRect(0, 0, overlay.width, overlay.height);
  }
  if (wrapper) wrapper.classList.remove('active');
}

async function captureAndSaveFace() {
  const empId = document.getElementById('registerFaceEmpId')?.value;
  if (!empId || !state.currentDetectedDescriptor) {
    showToast(window._lang === 'vi' ? 'Chưa bắt được mẫu khuôn mặt' : 'No face vector detected', 'error');
    return;
  }

  const captureBtn = document.getElementById('captureFaceBtn');
  const statusMsg = document.getElementById('registerFaceStatusMsg');
  const origText = captureBtn.innerHTML;
  captureBtn.disabled = true;
  captureBtn.innerHTML = '⏳ Đang lưu...';

  try {
    const descriptorJson = JSON.stringify(state.currentDetectedDescriptor);
    const res = await api(`/api/v1/employees/${empId}/face`, {
      method: 'POST',
      body: JSON.stringify({ face_descriptor: descriptorJson })
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed to register face');

    showToast(window._lang === 'vi' ? '✅ Đã lưu mẫu khuôn mặt thành công!' : '✅ Face registered successfully!', 'success');
    
    stopRegisterFaceCamera();
    const modal = document.getElementById('registerFaceModal');
    if (modal) modal.classList.remove('active');
    document.body.classList.remove('modal-open');

    loadEmployees().then(renderEmployees);
  } catch (err) {
    if (statusMsg) {
      statusMsg.className = 'message error';
      statusMsg.textContent = err.message;
    }
    showToast(err.message, 'error');
  } finally {
    if (captureBtn) {
      captureBtn.disabled = false;
      captureBtn.innerHTML = origText;
    }
  }
}
window.captureAndSaveFace = captureAndSaveFace;

function showFaceAttendanceToast(data) {
  const toast = document.getElementById('faceMatchToast');
  const avatar = document.getElementById('toastAvatar');
  const nameEl = document.getElementById('toastName');
  const codeEl = document.getElementById('toastCode');
  const timeEl = document.getElementById('toastTime');

  if (!toast) return;

  if (nameEl) nameEl.textContent = data.full_name || 'Nhân viên';
  if (codeEl) codeEl.textContent = data.employee_code || '';
  if (timeEl) timeEl.textContent = data.check_time || new Date().toLocaleTimeString();

  if (avatar) {
    if (data.avatar_url) {
      avatar.style.backgroundImage = `url("${data.avatar_url}")`;
      avatar.textContent = '';
    } else {
      avatar.style.backgroundImage = 'none';
      avatar.textContent = getEmployeeInitials(data.full_name || '?');
    }
  }

  updateLastScanCard(data);

  toast.style.display = 'block';

  try {
    const audioCtx = new (window.AudioContext || window.webkitAudioContext)();
    const osc = audioCtx.createOscillator();
    const gain = audioCtx.createGain();
    osc.type = 'sine';
    osc.frequency.setValueAtTime(587.33, audioCtx.currentTime);
    osc.frequency.exponentialRampToValueAtTime(880, audioCtx.currentTime + 0.15);
    gain.gain.setValueAtTime(0.1, audioCtx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.01, audioCtx.currentTime + 0.3);
    osc.connect(gain);
    gain.connect(audioCtx.destination);
    osc.start();
    osc.stop(audioCtx.currentTime + 0.3);
  } catch (e) {}

  setTimeout(() => {
    toast.style.display = 'none';
  }, 4000);
}

function updateLastScanCard(data) { renderFullLastScanCard(data); }

function appendFaceAttendanceLogRow(data) {
  const tbody = document.getElementById('faceAttendanceLogBody');
  if (!tbody || !data) return;

  const empCode = data.employee_code || data.latest_employee_code || '';
  const checkTime = data.check_time || data.latest_check_time || '';
  const empName = data.full_name || data.latest_employee_name || 'Nhân viên';

  // Chống ghi lặp hàng trong bảng nếu nhận từ cả REST API và SSE cùng lúc
  const rowKey = `${empCode}_${checkTime}_${empName}`;
  if (!state.recentLogRowKeys) state.recentLogRowKeys = {};
  if (state.recentLogRowKeys[rowKey]) return;
  state.recentLogRowKeys[rowKey] = true;
  setTimeout(() => { delete state.recentLogRowKeys[rowKey]; }, 5000);

  if (tbody.children.length === 1 && tbody.children[0].textContent.includes('Chưa có')) {
    tbody.innerHTML = '';
  }

  const tr = document.createElement('tr');
  tr.innerHTML = `
    <td>
      <div style="display:flex; align-items:center; gap:8px;">
        <span style="width:28px; height:28px; border-radius:50%; background:var(--accent); color:white; display:grid; place-items:center; font-size:0.75rem; font-weight:600;">
          ${getEmployeeInitials(empName)}
        </span>
        <strong>${empName}</strong>
      </div>
    </td>
    <td><code>${empCode || '-'}</code></td>
    <td>${checkTime || new Date().toLocaleTimeString()}</td>
    <td><span class="badge present">IN (Cam)</span></td>
  `;
  tbody.insertBefore(tr, tbody.firstChild);
}
window.deleteFaceFromRow = async function deleteFaceFromRow(employeeId) {
  const emp = (state.employeeMap && state.employeeMap[employeeId]) || (state.employees && state.employees.find(e => e.id === employeeId));
  const empName = emp ? emp.full_name : (window._lang === 'vi' ? 'nhân viên này' : 'this employee');
  
  const confirmMsg = window._lang === 'vi'
    ? `Bạn có chắc chắn muốn xóa dữ liệu khuôn mặt của ${empName}?`
    : `Are you sure you want to delete face data for ${empName}?`;

  if (!confirm(confirmMsg)) return;

  try {
    const res = await api(`/api/v1/employees/${employeeId}/face`, { method: 'DELETE' });
    if (!res.ok) {
      const data = await res.json();
      throw new Error(data.error || 'Failed');
    }
    showToast(window._lang === 'vi' ? `Đã xóa thành công khuôn mặt của ${empName}` : `Deleted face data for ${empName}`, 'success');
    await loadEmployees();
    await loadRegisteredFaces();
    renderEmployees();
  } catch (err) {
    showToast(err.message, 'error');
  }
};


function renderFullLastScanCard(data) {
  if (!data) return;

  state.lastScan = data;
  let emp = null;
  const empId = data.employee_id || data.latest_employee_id;
  const empCode = data.employee_code || data.latest_employee_code;

  if (empCode && state.employeeMap) emp = state.employeeMap[empCode];
  if (!emp && empId && state.employees) emp = state.employees.find(e => e.id === empId);
  if (!emp && empCode && state.employees) emp = state.employees.find(e => e.employee_code === empCode);

  const empName = emp?.full_name || data.full_name || data.latest_employee_name || (window._lang === 'vi' ? 'Chưa tìm thấy NV' : 'Unknown');
  const codeVal = emp?.employee_code || empCode || '—';
  
  const isCheckOut = (data.check_type || data.latest_check_type || '').toLowerCase() === 'out';
  const verifyMode = data.verify_mode || data.latest_verify_mode || 'fingerprint';
  const isFingerprint = verifyMode === 'fingerprint';

  const scanLabel = isFingerprint
    ? (window._lang === 'vi' ? '● NGƯỜI VỪA QUÉT VÂN TAY' : '● LATEST FINGERPRINT SCAN')
    : (window._lang === 'vi' ? '● NGƯỜI VỪA QUÉT KHUÔN MẶT' : '● LATEST FACE SCAN');

  const nameEl = document.getElementById('lastScanName');
  const codeEl = document.getElementById('lastScanCode');
  const labelEl = document.getElementById('lastScanLabel');
  const typeEl = document.getElementById('lastScanType');
  const timeEl = document.getElementById('lastScanTime');
  const deviceEl = document.getElementById('lastScanDevice');
  const avatarEl = document.getElementById('lastScanAvatar');
  const cardEl = document.getElementById('lastScanCard');

  if (nameEl) nameEl.textContent = empName;
  if (codeEl) codeEl.textContent = `${window._lang === 'vi' ? 'Mã nhân viên' : 'Employee code'}: ${codeVal}`;
  if (labelEl) labelEl.textContent = scanLabel;
  if (typeEl) {
    typeEl.textContent = isCheckOut ? (window._lang === 'vi' ? 'Ra về' : 'Check Out') : (window._lang === 'vi' ? 'Vào làm' : 'Check In');
    typeEl.className = `last-scan-type ${isCheckOut ? 'out' : 'in'}`;
  }
  if (timeEl) {
    const rawTime = data.check_time || data.latest_check_time;
    const t = rawTime ? new Date(rawTime) : new Date();
    timeEl.textContent = !isNaN(t.getTime()) ? t.toLocaleString(window._lang === 'vi' ? 'vi-VN' : 'en-US') : (rawTime || '—');
  }
  if (deviceEl) deviceEl.textContent = data.device_name || data.latest_device_name || (verifyMode === 'face' ? 'Camera Laptop' : 'Máy chấm công');

  if (avatarEl) {
    const avatarUrl = emp?.avatar_url || data.avatar_url;
    avatarEl.replaceChildren();
    if (avatarUrl) {
      const img = document.createElement('img');
      img.src = avatarUrl;
      img.alt = empName;
      img.addEventListener('error', () => { avatarEl.textContent = getEmployeeInitials(empName); }, { once: true });
      avatarEl.appendChild(img);
    } else {
      avatarEl.textContent = getEmployeeInitials(empName);
    }
  }

  // Ghi đầy đủ các trường dữ liệu
  const setChip = (id, val) => {
    const el = document.getElementById(id);
    if (el) el.textContent = val || '—';
  };

  setChip('scanDept', emp?.department_id || (window._lang === 'vi' ? 'Chưa phân phòng' : 'Unassigned'));
  setChip('scanJob', emp?.job_title || (window._lang === 'vi' ? 'Nhân viên' : 'Employee'));
  setChip('scanPhone', emp?.phone || (window._lang === 'vi' ? 'Chưa có SĐT' : 'No phone'));
  setChip('scanEmail', emp?.email || (window._lang === 'vi' ? 'Chưa có Email' : 'No email'));
  setChip('scanCardNo', emp?.card_no || (window._lang === 'vi' ? 'Chưa cấp thẻ' : 'No card'));
  setChip('scanGender', emp?.gender || (window._lang === 'vi' ? 'Chưa cập nhật' : 'N/A'));
  
  const dobText = emp?.dob ? new Date(emp.dob).toLocaleDateString(window._lang === 'vi' ? 'vi-VN' : 'en-US') : '—';
  const joinText = emp?.join_date ? new Date(emp.join_date).toLocaleDateString(window._lang === 'vi' ? 'vi-VN' : 'en-US') : '—';
  setChip('scanDob', dobText);
  setChip('scanJoinDate', joinText);
  setChip('scanZalo', emp?.zalo_user_id || '—');

  const fpText = emp?.fingerprint_enrolled ? (window._lang === 'vi' ? '🖐 Đã có' : 'Enrolled') : (window._lang === 'vi' ? '⚠ Chưa có' : 'Not enrolled');
  const faceText = emp?.face_enrolled ? (window._lang === 'vi' ? '📷 Đã có' : 'Enrolled') : (window._lang === 'vi' ? '⚠ Chưa có' : 'Not enrolled');
  const statusText = (emp?.status || 'active') === 'active' ? (window._lang === 'vi' ? '🟢 Hoạt động' : 'Active') : (window._lang === 'vi' ? '🔴 Tạm ngưng' : 'Inactive');

  setChip('scanFpStatus', fpText);
  setChip('scanFaceStatus', faceText);
  setChip('scanEmpStatus', statusText);

  if (cardEl) {
    cardEl.classList.remove('is-waiting');
    cardEl.classList.add('highlight-pulse');
    setTimeout(() => cardEl.classList.remove('highlight-pulse'), 1500);
  }
}

function showLastScan(scan) {
  renderFullLastScanCard(scan);
}

function updateLastScanCard(data) {
  renderFullLastScanCard(data);
}

window.renderFullLastScanCard = renderFullLastScanCard;
