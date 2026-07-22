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
  selectedFingerprintEmployeeId: null,
  selectedFingerprintFingerIndex: null,
  fingerprintTemplates: [],
  shifts: [],
  essRequests: []
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

const els = {};
let eventSource = null;

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
        const deviceName = d.device_name || 'máy chấm công';
        const empCode = d.latest_employee_code ? ` (NV: ${d.latest_employee_code})` : '';
        let checkTimeStr = '';
        if (d.latest_check_time) {
          try { checkTimeStr = ` lúc ${new Date(d.latest_check_time).toLocaleTimeString('vi-VN')}`; } catch(e){}
        }
        let msg;
        if (d.inserted > 0) {
          msg = `\u26a1 Quét thành công${empCode}${checkTimeStr} từ "${deviceName}" — Đã lưu ${d.inserted} log mới!`;
        } else {
          msg = `\u26a1 Nhận log từ "${deviceName}"${empCode}${checkTimeStr} (đã tồn tại, reload dữ liệu...)`;
        }
        showToast(msg, d.inserted > 0 ? 'success' : 'info');
        // Luôn tải lại để bảng chấm công cập nhật ngay
        loadAttendance().then(() => renderAttendance()).catch(()=>{});
      } else if (payload.type === 'attendance_processed') {
        const msg = window._lang === 'vi'
          ? `\u26a1 H\u1ec7 th\u1ed1ng v\u1eeba ho\u00e0n t\u1ea5t t\u00ednh to\u00e1n c\u00f4ng cho ng\u00e0y ${payload.data.date}!`
          : `\u26a1 System just finished calculating attendance for date ${payload.data.date}!`;
        showToast(msg, 'success');
        loadAll();
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
      } else if (payload.type === 'batch_enroll_queued') {
        const d = payload.data || {};
        const msg = window._lang === 'vi'
          ? `🖐 Đã đưa ${d.enqueued}/${d.total_requested} lệnh quét vân tay vào hàng đợi máy "${d.device_name}"!`
          : `🖐 Queued ${d.enqueued}/${d.total_requested} enroll commands to "${d.device_name}"!`;
        showToast(msg, 'success');
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

document.addEventListener('DOMContentLoaded', () => {
  bindElements();
  bindEvents();
  if (state.token) {
    showApp();
    loadAll();
    connectSSE();
    populateStopBatchDeviceSelect();
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

  // Reports elements
  els.reportsView = document.getElementById('reportsView');
  els.reportMonth = document.getElementById('reportMonth');
  els.applyReportBtn = document.getElementById('applyReportBtn');
  els.reportTableHeader = document.getElementById('reportTableHeader');
  els.reportTableBody = document.getElementById('reportTableBody');
  els.exportExcelBtn = document.getElementById('exportExcelBtn');
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
  els.closeEmployeeModalBtn = document.getElementById('closeEmployeeModalBtn');
  els.employeeModal = document.getElementById('employeeModal');
  els.employeeModalTitle = document.getElementById('employeeModalTitle');
  els.employeeSearch = document.getElementById('employeeSearch');

  els.openNewShiftModalBtn = document.getElementById('openNewShiftModalBtn');
  els.closeShiftModalBtn = document.getElementById('closeShiftModalBtn');
  els.closeShiftModalCancelBtn = document.getElementById('closeShiftModalCancelBtn');
  els.shiftModal = document.getElementById('shiftModal');

  els.openAssignShiftModalBtn = document.getElementById('openAssignShiftModalBtn');
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
  els.confirmSyncSingleEmployeeBtn = document.getElementById('confirmSyncSingleEmployeeBtn');
}

function bindEvents() {
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
  els.openNewShiftModalBtn.addEventListener('click', () => els.shiftModal.classList.add('active'));
  els.closeShiftModalBtn.addEventListener('click', () => els.shiftModal.classList.remove('active'));
  els.closeShiftModalCancelBtn.addEventListener('click', () => els.shiftModal.classList.remove('active'));
  els.shiftForm.addEventListener('submit', onSaveShift);
  els.openAssignShiftModalBtn.addEventListener('click', () => els.assignShiftModal.classList.add('active'));
  els.closeAssignShiftModalBtn.addEventListener('click', () => els.assignShiftModal.classList.remove('active'));
  els.closeAssignShiftModalCancelBtn.addEventListener('click', () => els.assignShiftModal.classList.remove('active'));
  els.assignShiftForm.addEventListener('submit', onAssignShift);
  els.deviceSearch.addEventListener('input', renderDevices);
  els.employeeSearch.addEventListener('input', renderEmployees);
  els.openNewEssModalBtn.addEventListener('click', () => {
    els.essForm.reset();
    toggleEssFormFields();
    els.essModal.classList.add('active');
  });
  els.closeEssModalBtn.addEventListener('click', () => els.essModal.classList.remove('active'));
  els.closeEssModalCancelBtn.addEventListener('click', () => els.essModal.classList.remove('active'));
  els.loadAttendanceBtn.addEventListener('click', loadAttendance);
  els.applyAttendanceBtn.addEventListener('click', loadAttendance);
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
  els.refreshAuditBtn.addEventListener('click', loadAuditLogs);
  els.essForm.addEventListener('submit', onSubmitEssRequest);
  document.addEventListener('click', onTableAction);

  // Fingerprint Modal events
  els.closeFingerprintModalBtn.addEventListener('click', () => els.fingerprintModal.classList.remove('active'));
  els.fpTriggerEnrollBtn.addEventListener('click', triggerRemoteEnroll);
  if (els.fpReEnrollBtn) els.fpReEnrollBtn.addEventListener('click', reEnrollFingerprint);
  els.fpPushToDevicesBtn.addEventListener('click', pushFingerprintsToAll);
  els.fpDeleteBtn.addEventListener('click', deleteFingerprint);
  els.fpManualConfirmLink.addEventListener('click', (e) => {
    e.preventDefault();
    if (state.selectedFingerprintEmployeeId) {
      confirmFingerprintEnrollment(state.selectedFingerprintEmployeeId);
    }
  });

  // New sync modal events
  const pushAllToDeviceBtn = document.getElementById('pushAllToDeviceBtn');
  const closePushAllModalBtn = document.getElementById('closePushAllModalBtn');
  const cancelPushAllBtn = document.getElementById('cancelPushAllBtn');
  const confirmPushAllBtn = document.getElementById('confirmPushAllBtn');
  const pushAllToDeviceModal = document.getElementById('pushAllToDeviceModal');
  if (pushAllToDeviceBtn) pushAllToDeviceBtn.addEventListener('click', openPushAllToDeviceModal);
  if (closePushAllModalBtn) closePushAllModalBtn.addEventListener('click', () => pushAllToDeviceModal.classList.remove('active'));
  if (cancelPushAllBtn) cancelPushAllBtn.addEventListener('click', () => pushAllToDeviceModal.classList.remove('active'));
  if (confirmPushAllBtn) confirmPushAllBtn.addEventListener('click', confirmPushAllToDevice);

  const batchEnrollBtn = document.getElementById('batchEnrollBtn');
  const batchEnrollAllBtn = document.getElementById('batchEnrollAllBtn');
  const closeBatchEnrollModalBtn = document.getElementById('closeBatchEnrollModalBtn');
  const cancelBatchEnrollBtn = document.getElementById('cancelBatchEnrollBtn');
  const confirmBatchEnrollBtn = document.getElementById('confirmBatchEnrollBtn');
  const stopBatchEnrollBtn = document.getElementById('stopBatchEnrollBtn');
  const batchEnrollModal = document.getElementById('batchEnrollModal');
  const batchEnrollSelectAll = document.getElementById('batchEnrollSelectAll');
  if (batchEnrollBtn) batchEnrollBtn.addEventListener('click', toggleSelectEnroll);
  if (batchEnrollAllBtn) batchEnrollAllBtn.addEventListener('click', runBatchEnrollForPendingEmployees);
  // Fallback: if elements weren't found at bind time (rare), use delegated listeners
  if (!batchEnrollBtn) {
    console.warn('batchEnrollBtn not found during bind, adding delegated listener');
    document.addEventListener('click', (e) => {
      const target = e.target || e.srcElement;
      if (target && target.id === 'batchEnrollBtn') {
        toggleSelectEnroll();
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
  if (stopBatchEnrollBtn) stopBatchEnrollBtn.addEventListener('click', stopBatchEnroll);
  if (batchEnrollSelectAll) batchEnrollSelectAll.addEventListener('change', (e) => {
    document.querySelectorAll('#batchEnrollEmployeeList input[type="checkbox"]').forEach(cb => cb.checked = e.target.checked);
  });

  const pullFromDeviceBtn = document.getElementById('pullFromDeviceBtn');
  const closePullFromDeviceModalBtn = document.getElementById('closePullFromDeviceModalBtn');
  const cancelPullFromDeviceBtn = document.getElementById('cancelPullFromDeviceBtn');
  const confirmPullFromDeviceBtn = document.getElementById('confirmPullFromDeviceBtn');
  const pullFromDeviceModal = document.getElementById('pullFromDeviceModal');
  if (pullFromDeviceBtn) pullFromDeviceBtn.addEventListener('click', openPullFromDeviceModal);
  if (els.pushAllNowBtn) els.pushAllNowBtn.addEventListener('click', runPushAllToDeviceNow);
  if (els.pullFromDeviceNowBtn) els.pullFromDeviceNowBtn.addEventListener('click', runPullFromDeviceNow);
  if (closePullFromDeviceModalBtn) closePullFromDeviceModalBtn.addEventListener('click', () => pullFromDeviceModal.classList.remove('active'));
  if (cancelPullFromDeviceBtn) cancelPullFromDeviceBtn.addEventListener('click', () => pullFromDeviceModal.classList.remove('active'));
  if (confirmPullFromDeviceBtn) confirmPullFromDeviceBtn.addEventListener('click', confirmPullFromDevice);

  // Sync Single Employee Modal events
  if (els.closeSyncSingleEmployeeModalBtn) els.closeSyncSingleEmployeeModalBtn.addEventListener('click', () => els.syncSingleEmployeeModal.classList.remove('active'));
  if (els.cancelSyncSingleEmployeeBtn) els.cancelSyncSingleEmployeeBtn.addEventListener('click', () => els.syncSingleEmployeeModal.classList.remove('active'));
  if (els.confirmSyncSingleEmployeeBtn) els.confirmSyncSingleEmployeeBtn.addEventListener('click', confirmSyncSingleEmployee);

  // Sync Toolbar Dropdown
  const syncDropdownBtn = document.getElementById('syncDropdownBtn');
  const syncDropdownContent = document.getElementById('syncDropdownContent');
  if (syncDropdownBtn && syncDropdownContent) {
    syncDropdownBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      syncDropdownContent.classList.toggle('active');
    });
  }

  // Click Outside to Close all Dropdowns
  document.addEventListener('click', () => {
    document.querySelectorAll('.row-dropdown-content').forEach(d => d.classList.remove('active'));
    if (syncDropdownContent) syncDropdownContent.classList.remove('active');
  });

  // Conditionally Show Excel Import Button
  const employeeFileInput = document.getElementById('employeeFileInput');
  const importEmployeesBtn = document.getElementById('importEmployeesBtn');
  if (employeeFileInput && importEmployeesBtn) {
    employeeFileInput.addEventListener('change', () => {
      if (employeeFileInput.files.length > 0) {
        importEmployeesBtn.style.display = 'inline-block';
      } else {
        importEmployeesBtn.style.display = 'none';
      }
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
    settings: { vi: 'Cài đặt', en: 'Settings' }
  };

  if (els.pageTitle) {
    const lang = window._lang === 'en' ? 'en' : 'vi';
    els.pageTitle.textContent = titles[viewName]?.[lang] || (lang === 'en' ? 'Dashboard' : 'Tổng quan');
  }

  if (viewName === 'ess') {
    loadEssInitialData().catch(() => {});
  } else if (viewName === 'attendance') {
    loadAttendance().catch(() => {});
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
  event.preventDefault();
  const username = els.username.value.trim();
  const password = els.password.value;
  try {
    const response = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || (window._lang === 'vi' ? '\u0110\u0103ng nh\u1eadp th\u1ea5t b\u1ea1i' : 'Login failed'));
    state.token = data.token;
    state.user = { username };
    localStorage.setItem('attendance-token', state.token);
    localStorage.setItem('attendance-user', JSON.stringify(state.user));
    showApp();
    loadAll();
    connectSSE();
    els.loginMessage.textContent = window._lang === 'vi' ? '\u0110\u0103ng nh\u1eadp th\u00e0nh c\u00f4ng' : 'Login successful';
  } catch (error) {
    els.loginMessage.textContent = error.message;
    els.loginMessage.classList.add('error');
  }
}

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
  const admsDevices = state.devices.filter(d => d.adms_enabled);
  if (admsDevices.length === 0) {
    els.employeeEnrollDeviceId.innerHTML = '<option value="">-- Không có thiết bị ADMS --</option>';
    return;
  }
  els.employeeEnrollDeviceId.innerHTML = admsDevices.map(d => `<option value="${d.id}">${d.name} (${d.ip_address || 'N/A'})</option>`).join('');
}

function populateSyncDeviceSelect() {
  const select = els.syncDeviceSelect;
  if (!select) return;
  const admsDevices = (state.devices || []).filter(d => d && d.adms_enabled);
  if (admsDevices.length === 0) {
    select.innerHTML = '<option value="">-- Không có thiết bị ADMS --</option>';
    return;
  }
  select.innerHTML = ['<option value="">-- Chọn thiết bị để đồng bộ --</option>'].concat(admsDevices.map(d => `<option value="${d.id}">${d.name} (${d.ip_address || '-'}) ${d.status === 'online' ? '✅' : '⚠'}</option>`)).join('');
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
    if (!device.adms_enabled) throw new Error(window._lang === 'vi' ? 'Thiết bị không hỗ trợ ADMS' : 'Device does not support ADMS');
    if (device.status !== 'online') throw new Error(window._lang === 'vi' ? 'Thiết bị hiện không online, không thể đẩy dữ liệu' : 'Device is not online; cannot push data');
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
        // Timeout 25s mỗi thiết bị - thiết bị offline sẽ fail nhanh thay vì treo
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 3000);
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
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || 'Failed to delete device');
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

async function openFingerprintModal(employeeId, autoEnroll = false) {
  state.selectedFingerprintEmployeeId = employeeId;
  const emp = state.employees.find(e => e.id === employeeId);
  if (!emp) return;

  els.fpModalEmployeeName.textContent = emp.full_name;
  els.fpModalEmployeeCode.textContent = emp.employee_code;
  
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
      els.fpFingerprintList.style.display = 'none';
      els.fpFingerprintListItems.innerHTML = '';
    }
    const enrollDevices = state.devices.filter(d => d.status === 'online' && (d.adms_enabled ? d.serial_number_adms : true));
    if (hasFp) {
      els.fpModalStatusBadge.textContent = window._lang === 'vi' ? 'Đã đăng ký (1 ngón)' : 'Registered (1 finger)';
      els.fpModalStatusBadge.className = 'badge online';
      if (els.fpActionsSection) els.fpActionsSection.style.display = 'flex';
      if (els.fpReEnrollBtn) els.fpReEnrollBtn.style.display = 'inline-flex';
      if (enrollDevices.length > 0) {
        if (els.fpEnrollSection) els.fpEnrollSection.style.display = 'block';
        els.fpEnrollDeviceId.innerHTML = enrollDevices.map(d => 
          `<option value="${d.id}">${d.name} (${d.ip_address})</option>`
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
          `<option value="${d.id}">${d.name} (${d.ip_address})</option>`
        ).join('');
        els.fpTriggerEnrollBtn.disabled = false;
      } else {
        els.fpEnrollDeviceId.innerHTML = `<option value="">${window._lang === 'vi' ? '-- Không có thiết bị trực tuyến hoặc cấu hình thiếu --' : '-- No online devices available or configuration missing --'}</option>`;
        els.fpTriggerEnrollBtn.disabled = true;
      }
    }
    
    els.fingerprintModal.classList.add('active');
    if (autoEnroll) {
      await triggerRemoteEnroll();
    }
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function enrollFingerprintFromRow(employeeId) {
  await openFingerprintModal(employeeId, true);
}

async function triggerRemoteEnroll() {
  const employeeId = state.selectedFingerprintEmployeeId;
  const deviceId = els.fpEnrollDeviceId.value;
  if (!deviceId) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị quét!' : 'Please select an enroll device!', 'error');
    return;
  }

  const device = state.devices.find(d => d.id === deviceId);
  const deviceName = device ? device.name : '';

  try {
    els.fpTriggerEnrollBtn.disabled = true;
    const response = await api(`/api/v1/employees/${employeeId}/fingerprints/enroll`, {
      method: 'POST',
      body: JSON.stringify({ device_id: deviceId })
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

async function reEnrollFingerprint() {
  const employeeId = state.selectedFingerprintEmployeeId;
  if (!employeeId) {
    showToast(window._lang === 'vi' ? 'Không xác định được nhân viên để đăng ký lại vân tay.' : 'Employee not specified for fingerprint re-enroll.', 'error');
    return;
  }

  const deviceId = els.fpEnrollDeviceId.value;
  if (!deviceId) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị để đăng ký lại!' : 'Please select a device to re-enroll!', 'error');
    return;
  }

  try {
    els.fpReEnrollBtn.disabled = true;
    const response = await api(`/api/v1/employees/${employeeId}/fingerprints/re-enroll`, {
      method: 'POST',
      body: JSON.stringify({ device_id: deviceId })
    });
    const data = await readJsonResponse(response);
    if (!response.ok) throw new Error(data?.error || data?.message || 'Failed to trigger re-enroll');

    const device = state.devices.find(d => d.id === deviceId);
    const deviceName = device ? device.name : '';
    showToast(window._lang === 'vi'
      ? `Đã gửi yêu cầu đăng ký lại tới ${deviceName || 'máy chấm công'}; đang chờ máy xác nhận và gửi dữ liệu vân tay về web.`
      : `Re-enrollment request queued for ${deviceName || 'biometric device'}; waiting for the device confirmation and fingerprint data.`, 'info');
    els.fpInstructionBox.style.display = 'block';
  } catch (error) {
    showToast(error.message, 'error');
    throw error;
  } finally {
    els.fpReEnrollBtn.disabled = false;
  }
}

function renderFingerprintTemplates(fps) {
  if (!els.fpFingerprintListItems) return;
  els.fpFingerprintListItems.innerHTML = fps.map(fp => {
    const created = fp.created_at ? ` • ${new Date(fp.created_at).toLocaleString('vi-VN')}` : '';
    return `
      <div class="fp-item" style="display:flex; justify-content:space-between; align-items:center; gap: 10px; padding:10px; border:1px solid var(--border); border-radius:8px; background: rgba(255,255,255,0.02);">
        <div>
          <div style="font-size:0.95rem; font-weight:600;">Ngón #${fp.finger_index + 1} (${fp.finger_index})</div>
          <div style="font-size:0.85rem; color: var(--muted);">Kích thước: ${fp.template_size} bytes${created}</div>
        </div>
        <button type="button" class="secondary-btn" style="padding: 8px 12px; min-width: 120px; font-size:0.85rem;" onclick="deleteFingerprint(${fp.finger_index})">
          🗑️ Xóa
        </button>
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

    // Badge vân tay
    const fpEnrolled = employee.fingerprint_enrolled;
    const fpBadgeHtml = fpEnrolled
      ? `<span class="badge online" style="font-size:0.75rem; padding: 3px 8px;">🖐 Đã có</span>`
      : `<span class="badge offline" style="font-size:0.75rem; padding: 3px 8px;">⚠ Chưa có</span>`;

    return `
      <tr>
        ${state.batchSelectMode ? `<td style="width:40px; text-align:center;"><input type="checkbox" onchange="toggleEmployeeSelection('${employee.id}', this.checked)" ${state.batchSelected[employee.id] ? 'checked' : ''} /></td>` : ''}
        <td><code>${employee.employee_code}</code></td>
        <td>${infoHtml}</td>
        <td>${deptTitleHtml}</td>
        <td><span class="muted" style="font-size:0.9rem;">${joinDateText}</span></td>
        <td>${fpBadgeHtml}</td>
        <td><span class="badge ${statusClass}">${statusText}</span></td>
        <td>
          <div class="row-dropdown">
            <button type="button" class="row-dropdown-btn" onclick="toggleRowDropdown(event, '${employee.id}')">⋮</button>
            <div class="row-dropdown-content" id="dropdown-${employee.id}">
              <button type="button" onclick="event.stopPropagation(); this.parentElement.classList.remove('active'); state.selectedFingerprintEmployeeId='${employee.id}'; openFingerprintModal('${employee.id}');">🖐️ ${window._lang === 'vi' ? 'Quản lý vân tay' : 'Manage fingerprint'}</button>
              <button type="button" onclick="event.stopPropagation(); this.parentElement.classList.remove('active'); state.selectedFingerprintEmployeeId='${employee.id}'; enrollFingerprintFromRow('${employee.id}');">⚡ ${window._lang === 'vi' ? 'Đăng ký vân tay' : 'Enroll fingerprint'}</button>
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
        <td>${otDisplay}</td>
        <td>${leaveDisplay}</td>
        <td><span class="badge ${statusClass}">${statusText}</span></td>
      </tr>`;
  }).join('') : `<tr><td colspan="7">${window._lang === 'vi' ? 'Không có dữ liệu tóm tắt ngày hôm nay' : 'No summary data for today'}</td></tr>`;

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
    return `
      <tr>
        <td>${empDisplay}</td>
        <td><code>${new Date(item.check_time).toLocaleString(window._lang === 'vi' ? 'vi-VN' : 'en-US')}</code></td>
        <td><span class="badge ${typeClass}">${typeText}</span></td>
        <td>${devDisplay}</td>
      </tr>`;
  }).join('');
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
    const response = await api('/api/v1/shifts');
    const data = await response.json();
    state.shifts = Array.isArray(data) ? data : [];
  } catch {
    state.shifts = [];
  }
}

function renderShifts() {
  // Render Shift Table
  els.shiftTableBody.innerHTML = state.shifts.map((s) => `
    <tr>
      <td>${s.name}</td>
      <td>${s.start_time}</td>
      <td>${s.end_time}</td>
      <td>
        <button class="secondary-btn" data-action="delete-shift" data-id="${s.id}">${t('btnDelete')}</button>
      </td>
    </tr>`).join('');

  // Update employee select dropdown
  els.assignEmployeeId.innerHTML = state.employees.map((e) => `
    <option value="${e.id}">${e.full_name} (${e.employee_code})</option>`).join('');

  // Update shift select dropdown
  els.assignShiftId.innerHTML = state.shifts.map((s) => `
    <option value="${s.id}">${s.name} (${s.start_time} - ${s.end_time})</option>`).join('');

  // Default start date to today
  if (!els.assignStartDate.value) {
    els.assignStartDate.value = getLocalDateStr(new Date());
  }
}

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
    const response = await api('/api/v1/shifts', { method: 'POST', body: JSON.stringify(payload) });
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Không thể lưu ca' : 'Failed to save shift');
    showToast(t('toastCreateShiftSuccess'), 'success');
    els.shiftForm.reset();
    els.shiftModal.classList.remove('active'); // Đóng modal
    await loadShifts();
    renderShifts();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function onAssignShift(event) {
  event.preventDefault();
  const employeeId = els.assignEmployeeId.value;
  const payload = {
    shift_id: els.assignShiftId.value,
    start_date: els.assignStartDate.value
  };
  try {
    const response = await api(`/api/v1/employees/${employeeId}/shifts`, { method: 'POST', body: JSON.stringify(payload) });
    if (!response.ok) throw new Error(window._lang === 'vi' ? 'Không thể gán ca' : 'Failed to assign shift');
    showToast(t('toastAssignShiftSuccess'), 'success');
    els.assignShiftModal.classList.remove('active'); // Đóng modal
  } catch (error) {
    showToast(error.message, 'error');
  }
}

async function deleteShift(id) {
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
          labels: { color: '#f8fafc', font: { family: 'Outfit', size: 11 } }
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
        borderColor: '#6366f1',
        backgroundColor: 'rgba(99, 102, 241, 0.15)',
        fill: true,
        tension: 0.4,
        borderWidth: 3
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      scales: {
        x: { grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#94a3b8', font: { family: 'Outfit' } } },
        y: { grid: { color: 'rgba(255,255,255,0.05)' }, ticks: { color: '#94a3b8', font: { family: 'Outfit' } } }
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
  
  const closeModal = () => modal.classList.remove('active');
  
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
    console.log('batchEnroll modal opened (active class added)');
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

async function loadEssInitialData() {
  // Populate employees dropdown
  if (state.employees.length === 0) {
    await loadEmployees();
  }
  els.essEmployeeId.innerHTML = state.employees.map(e => 
    `<option value="${e.id}">${e.full_name} (${e.employee_code})</option>`
  ).join('');

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
      statusText = window._lang === 'vi' ? 'Đã duyệt' : 'Approved';
    } else if (item.status === 'rejected') {
      statusText = window._lang === 'vi' ? 'Từ chối' : 'Rejected';
    } else {
      statusText = window._lang === 'vi' ? 'Chờ duyệt' : 'Pending';
    }

    let actionsHtml = '-';
    if (item.status === 'pending') {
      actionsHtml = `
        <div style="display:flex; gap:4px;">
          <button class="primary-btn" style="padding:4px 8px; font-size:12px; background-color:#10b981; border:none;" data-action="approve-ess" data-type="${item.ess_type}" data-id="${item.id}">Duyệt</button>
          <button class="secondary-btn" style="padding:4px 8px; font-size:12px;" data-action="reject-ess" data-type="${item.ess_type}" data-id="${item.id}">Từ chối</button>
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
    payload.start_date = els.essLeaveStartDate.value + 'T00:00:00Z';
    payload.end_date = els.essLeaveEndDate.value + 'T23:59:59Z';
  } else if (type === 'overtime') {
    url = '/api/v1/overtime-requests';
    payload.date = els.essOtDate.value + 'T00:00:00Z';
    payload.start_time = els.essOtStartTime.value;
    payload.end_time = els.essOtEndTime.value;
  } else if (type === 'correction') {
    url = '/api/v1/attendance-corrections';
    payload.date = els.essCorrDate.value + 'T00:00:00Z';
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
// One-click toolbar actions use the first online ADMS device. The server also
// validates its heartbeat before it accepts a fingerprint enrollment request.
function getOnlineADMSDevice() {
  return state.devices.find(d => d.adms_enabled && d.status === 'online') || null;
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

  const admsDevices = state.devices.filter(d => d.adms_enabled);
  if (admsDevices.length === 0) {
    showToast(window._lang === 'vi' ? 'Không có thiết bị ADMS nào!' : 'No ADMS devices found!', 'warning');
    return;
  }
  select.innerHTML = admsDevices.map(d => {
    const online = d.status === 'online' ? ' ✅ Online' : ' ⚠ Offline';
    return `<option value="${d.id}">${d.name} (${d.ip_address || '-'})${online}</option>`;
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
  const admsDevices = (state.devices || []).filter(d => d && d.adms_enabled);
  if (admsDevices.length === 0) {
    select.innerHTML = '<option value="">-- Chưa có thiết bị ADMS --</option>';
    return;
  }
  select.innerHTML = ['<option value="">-- Chọn thiết bị --</option>'].concat(admsDevices.map(d => {
    const online = d.status === 'online' ? ' ✅' : ' ⚠';
    const label = d.name || d.ip_address || d.id;
    return `<option value="${d.id}">${online} ${label}</option>`;
  })).join('');
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

  const admsDevices = state.devices.filter(d => d.adms_enabled);
  if (admsDevices.length === 0) {
    showToast(window._lang === 'vi' ? 'Không có thiết bị ADMS nào!' : 'No ADMS devices!', 'warning');
    return;
  }
  deviceSelect.innerHTML = admsDevices.map(d => {
    const online = d.status === 'online' ? ' ✅' : ' ⚠';
    return `<option value="${d.id}">${online} ${d.name}</option>`;
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

async function stopBatchEnroll() {
  console.log('stopBatchEnroll invoked');
  const deviceSelect = document.getElementById('stopBatchEnrollDeviceSelect');
  const statusMsg = document.getElementById('batchEnrollStatusMsg');
  const btn = document.getElementById('stopBatchEnrollBtn');
  if (!deviceSelect || !deviceSelect.value) {
    showToast(window._lang === 'vi' ? 'Vui lòng chọn thiết bị trước khi dừng.' : 'Please select a device before stopping.', 'warning');
    return;
  }
  const original = btn.innerHTML;
  btn.disabled = true;
  btn.innerHTML = '⏳ Đang dừng...';
  try {
    const response = await api(`/api/v1/devices/${deviceSelect.value}/cancel-pending`, { method: 'POST' });
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
    if (d.id !== `dropdown-${employeeId}`) d.classList.remove('active');
  });
  const content = document.getElementById(`dropdown-${employeeId}`);
  if (content) {
    content.classList.toggle('active');
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

  const admsDevices = state.devices.filter(d => d.adms_enabled);
  if (admsDevices.length === 0) {
    showToast(window._lang === 'vi' ? 'Không có thiết bị ADMS nào!' : 'No ADMS devices found!', 'warning');
    return;
  }

  els.syncSingleEmployeeDeviceSelect.innerHTML = admsDevices.map(d => {
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

