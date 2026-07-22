package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"attendance-system/internal/domain/entity"
	"attendance-system/internal/domain/port"
)

func TestExtractPinFromADMSCommand(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"ENROLL_INFO PIN=123\tTYPE=1\tRETRY=3\tMODE=1", "123"},
		{"ENROLL_INFO PIN=ABC123 RETRY=3 MODE=1", "ABC123"},
		{"ENROLL_INFO TYPE=1\tPIN=0001\tMODE=1", "0001"},
		{"ENROLL_INFO PIN= \tTYPE=1", ""},
		{"DATA UPDATE user Pin=123\tName=Test", ""},
	}

	for _, c := range cases {
		got := extractPinFromADMSCommand(c.command)
		if got != c.want {
			t.Fatalf("extractPinFromADMSCommand(%q) = %q, want %q", c.command, got, c.want)
		}
	}
}

func TestBuildADMSFallbackEnrollCommand(t *testing.T) {
	got := buildADMSFallbackEnrollCommand("321", 0)
	want := "ENROLL_FP PIN=321\tFID=0\tRETRY=3\tOVERWRITE=1"
	if got != want {
		t.Fatalf("buildADMSFallbackEnrollCommand = %q, want %q", got, want)
	}
}

func TestParseADMSBiometricPayloadSupportsCommandPrefix(t *testing.T) {
	got := parseADMSBiometricPayload("DATA UPDATE FINGERTEMPLATE PIN=771\tFINGERID=1\tSIZE=160\tVAL=1\tTEMPLATE=dGVzdA==")
	if got == nil {
		t.Fatal("parseADMSBiometricPayload returned nil")
	}
	if got.Pin != "771" || got.FingerIndex != 1 || got.TemplateData != "dGVzdA==" || got.TemplateSize != 160 {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestInferADMSDataTableFromBiometricBody(t *testing.T) {
	if got := inferADMSDataTable("ATTLOG", "DATA UPDATE FINGERTEMPLATE PIN=771\tFINGERID=1\tSIZE=160\tVAL=1\tTEMPLATE=dGVzdA=="); got != "BIODATA" {
		t.Fatalf("inferADMSDataTable = %q, want %q", got, "BIODATA")
	}

	if got := inferADMSDataTable("", "DATA UPDATE BIODATA PIN=771\tFID=1\tTMP=dGVzdA=="); got != "BIODATA" {
		t.Fatalf("inferADMSDataTable = %q, want %q", got, "BIODATA")
	}

	if got := inferADMSDataTable("OPERLOG", "FP PIN=24\tFID=0\tSize=1736\tValid=1\tTMP=dGVzdA=="); got != "BIODATA" {
		t.Fatalf("inferADMSDataTable = %q, want %q", got, "BIODATA")
	}
}

func TestADMSFieldValueSupportsKeyValueAttendanceFields(t *testing.T) {
	if got := admsFieldValue("PIN=0012"); got != "0012" {
		t.Fatalf("admsFieldValue(PIN=0012) = %q, want 0012", got)
	}
	if got := admsFieldValue("2026-07-20 08:15:00"); got != "2026-07-20 08:15:00" {
		t.Fatalf("admsFieldValue(timestamp) = %q", got)
	}
}

func TestParseADMSBiometricPayloadSupportsFPFormat(t *testing.T) {
	got := parseADMSBiometricPayload("FP PIN=24\tFID=0\tSize=1736\tValid=1\tTMP=dGVzdA==")
	if got == nil {
		t.Fatal("parseADMSBiometricPayload returned nil")
	}
	if got.Pin != "24" || got.FingerIndex != 0 || got.TemplateData != "dGVzdA==" || got.TemplateSize != 1736 {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestParseADMSBiometricPayloadDerivesSizeAndRequiresFingerIndex(t *testing.T) {
	payload := parseADMSBiometricPayload("PIN=24\tFID=2\tTMP=dGVzdA==")
	if payload == nil {
		t.Fatal("expected payload")
	}
	if payload.TemplateSize != 4 { // decoded base64 value is "test"
		t.Fatalf("TemplateSize = %d, want 4", payload.TemplateSize)
	}

	if got := parseADMSBiometricPayload("PIN=24\tTMP=dGVzdA=="); got != nil {
		t.Fatalf("payload without a finger index must be rejected, got %+v", got)
	}
}

func TestIsADMSDeviceUsesExplicitProtocolFlag(t *testing.T) {
	d := &entity.Device{SerialNumberADMS: "8116255100515"}
	if isADMSDevice(d) {
		t.Fatal("a stored ADMS serial must not override an explicit SDK/Pull selection")
	}
	d.ADMSEnabled = true
	if !isADMSDevice(d) {
		t.Fatal("expected an explicitly enabled ADMS device to use ADMS")
	}
}

func TestNormalizeADMSPin(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"123", "123"},
		{"EMP001", "001"},
		{"EMP-002", "002"},
	}

	for _, tc := range cases {
		got := normalizeADMSPin(tc.input)
		if got != tc.want {
			t.Fatalf("normalizeADMSPin(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestBuildADMSUserCommandUsesCompatibleFormat(t *testing.T) {
	got := buildADMSUserCommand("EMP001", "Nguyen Van An", "0001234567")
	want := "DATA UPDATE USERINFO PIN=001\tName=Nguyen Van An\tPri=0\tPasswd=\tCard=0001234567\tGrp=1"
	if got != want {
		t.Fatalf("buildADMSUserCommand = %q, want %q", got, want)
	}
}

func TestBuildADMSFingerprintCommandUsesCompatibleFormat(t *testing.T) {
	got := buildADMSFingerprintCommand("EMP001", 6, 1616, "dummy")
	want := "DATA UPDATE BIODATA PIN=001\tNo=1\tIndex=6\tValid=1\tDuress=0\tType=9\tMajorVer=5\tMinorVer=8\tFormat=0\tTmp=dummy"
	if got != want {
		t.Fatalf("buildADMSFingerprintCommand = %q, want %q", got, want)
	}
}

func TestBuildADMSEnrollCommandUsesTerminalCompatibleFingerprintVariant(t *testing.T) {
	got := buildADMSEnrollCommand("EMP001", 0)
	want := "ENROLL_FP PIN=001\tFID=0\tRETRY=3\tOVERWRITE=1"
	if got != want {
		t.Fatalf("buildADMSEnrollCommand = %q, want %q", got, want)
	}
}

func TestBuildADMSUserCommandIncludesExtendedZKTecoFields(t *testing.T) {
	got := buildADMSUserCommand("EMP001", "Nguyen Van An", "0001234567")
	for _, want := range []string{"Passwd=", "Card=", "Grp=1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildADMSUserCommand = %q, want it to include %q", got, want)
		}
	}
}

func TestNextADMSFingerprintCommandVariantProvidesFallback(t *testing.T) {
	current := buildADMSFingerprintCommand("EMP001", 6, 1616, "dummy")
	got := nextADMSFingerprintCommandVariant("EMP001", 6, 1616, "dummy", current)
	if got == "" || got == current {
		t.Fatalf("expected an alternate fingerprint command variant, got %q", got)
	}
	if !strings.Contains(strings.ToUpper(got), "TEMPLATEV10") && !strings.Contains(strings.ToUpper(got), "FINGERTEMPLATE") && !strings.Contains(strings.ToUpper(got), "BIODATA") {
		t.Fatalf("expected fallback variant to remain fingerprint-based, got %q", got)
	}
	if !strings.Contains(strings.ToUpper(got), "FINGERID=") && !strings.Contains(strings.ToUpper(got), "FID=") && !strings.Contains(strings.ToUpper(got), "INDEX=") {
		t.Fatalf("expected fallback variant to carry a fingerprint field, got %q", got)
	}
	if !strings.Contains(got, "TMP=") && !strings.Contains(got, "TEMPLATE=") && !strings.Contains(got, "Tmp=") && !strings.Contains(got, "Template=") {
		t.Fatalf("expected fallback variant to carry a biometric payload, got %q", got)
	}
}

type stubEmployeeRepo struct {
	employees []entity.Employee
}

func (s *stubEmployeeRepo) Create(ctx context.Context, e *entity.Employee) error { return nil }
func (s *stubEmployeeRepo) Update(ctx context.Context, e *entity.Employee) error { return nil }
func (s *stubEmployeeRepo) Delete(ctx context.Context, id string) error          { return nil }
func (s *stubEmployeeRepo) DeleteAll(ctx context.Context) (int64, error)         { return 0, nil }
func (s *stubEmployeeRepo) GetByID(ctx context.Context, id string) (*entity.Employee, error) {
	return nil, nil
}
func (s *stubEmployeeRepo) GetByCode(ctx context.Context, code string) (*entity.Employee, error) {
	for i := range s.employees {
		if s.employees[i].EmployeeCode == code {
			return &s.employees[i], nil
		}
	}
	return nil, nil
}
func (s *stubEmployeeRepo) List(ctx context.Context) ([]entity.Employee, error) {
	out := make([]entity.Employee, len(s.employees))
	copy(out, s.employees)
	return out, nil
}
func (s *stubEmployeeRepo) ListActive(ctx context.Context) ([]entity.Employee, error) {
	return nil, nil
}

var _ port.EmployeeRepository = (*stubEmployeeRepo)(nil)

type stubMappingRepo struct {
	stored []entity.EmployeeDeviceMapping
}

func (s *stubMappingRepo) Upsert(ctx context.Context, mapping *entity.EmployeeDeviceMapping) error {
	s.stored = append(s.stored, *mapping)
	return nil
}
func (s *stubMappingRepo) ListByEmployee(ctx context.Context, employeeID string) ([]entity.EmployeeDeviceMapping, error) {
	return nil, nil
}
func (s *stubMappingRepo) ListByDevice(ctx context.Context, deviceID string) ([]entity.EmployeeDeviceMapping, error) {
	return nil, nil
}
func (s *stubMappingRepo) GetByEmployeeAndDevice(ctx context.Context, employeeID, deviceID string) (*entity.EmployeeDeviceMapping, error) {
	return nil, nil
}
func (s *stubMappingRepo) GetByDeviceUserID(ctx context.Context, deviceID, deviceUserID string) (*entity.EmployeeDeviceMapping, error) {
	for i := range s.stored {
		if s.stored[i].DeviceID == deviceID && s.stored[i].DeviceUserID == deviceUserID {
			m := s.stored[i]
			return &m, nil
		}
	}
	return nil, nil
}
func (s *stubMappingRepo) MarkFingerprintEnrolled(ctx context.Context, employeeID, deviceID string, enrolledAt time.Time) error {
	return nil
}

var _ port.EmployeeDeviceMappingRepository = (*stubMappingRepo)(nil)

func TestResolveEmployeeIDForADMSPinMatchesNormalizedCodes(t *testing.T) {
	repo := &stubEmployeeRepo{employees: []entity.Employee{{ID: "emp-1", EmployeeCode: "EMP001"}}}
	got, err := resolveEmployeeIDForADMSPin(context.Background(), repo, nil, "dev-1", "001")
	if err != nil {
		t.Fatalf("resolveEmployeeIDForADMSPin returned error: %v", err)
	}
	if got != "emp-1" {
		t.Fatalf("resolveEmployeeIDForADMSPin = %q, want %q", got, "emp-1")
	}
}

func TestMarkDeviceUserSyncedUsesNormalizedPin(t *testing.T) {
	repo := &stubEmployeeRepo{employees: []entity.Employee{{ID: "emp-1", EmployeeCode: "EMP001"}}}
	mappingRepo := &stubMappingRepo{}
	service := &ADMSService{employeeRepo: repo, mappingRepo: mappingRepo}

	if err := service.markDeviceUserSynced(context.Background(), "dev-1", "001"); err != nil {
		t.Fatalf("markDeviceUserSynced returned error: %v", err)
	}
	if len(mappingRepo.stored) != 1 {
		t.Fatalf("expected one stored mapping, got %d", len(mappingRepo.stored))
	}
	if mappingRepo.stored[0].EmployeeID != "emp-1" || mappingRepo.stored[0].DeviceUserID != "001" {
		t.Fatalf("unexpected mapping payload: %+v", mappingRepo.stored[0])
	}
}
