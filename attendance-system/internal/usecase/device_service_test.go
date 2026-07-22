package usecase

import (
	"strings"
	"testing"
)

func TestBuildZKTecoProtocolCapabilities(t *testing.T) {
	caps := buildProtocolCapabilities("zkteco", true, 4370)

	if len(caps) == 0 {
		t.Fatal("expected protocol capabilities for zkteco")
	}

	found := false
	for _, cap := range caps {
		if strings.Contains(cap, "Kết nối TCP/IP") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected tcp/ip capability, got %v", caps)
	}
}
