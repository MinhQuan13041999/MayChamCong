package http

import (
	"net/http/httptest"
	"testing"
)

func TestExtractADMSSerialSupportsCommonParameterNames(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{name: "upper SN", query: "SN=8116255100515", expected: "8116255100515"},
		{name: "lower sn", query: "sn=8116255100515", expected: "8116255100515"},
		{name: "serial", query: "serial=8116255100515", expected: "8116255100515"},
		{name: "mixed case", query: "sN=8116255100515", expected: "8116255100515"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/iclock/cdata?"+tt.query, nil)
			if got := extractADMSSerial(req); got != tt.expected {
				t.Fatalf("extractADMSSerial(%q) = %q, want %q", tt.query, got, tt.expected)
			}
		})
	}
}
