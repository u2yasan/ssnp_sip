package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/u2yasan/ssnp_sip/portal/internal/verify"
)

func TestDecodeObjectPreservesJSONNumberLexemesForAuthCanonical(t *testing.T) {
	req := httptest.NewRequest(
		"POST",
		"/",
		strings.NewReader(`{"normalized_cpu_score":123.0,"measured_latency_p95":0.0,"signature":"ignored"}`),
	)
	payload, err := decodeObject(req)
	if err != nil {
		t.Fatalf("decodeObject() error = %v", err)
	}
	canonical, err := verify.CanonicalJSONWithoutSignature(payload)
	if err != nil {
		t.Fatalf("CanonicalJSONWithoutSignature() error = %v", err)
	}
	want := `{"measured_latency_p95":0.0,"normalized_cpu_score":123.0}`
	if string(canonical) != want {
		t.Fatalf("canonical = %s, want %s", string(canonical), want)
	}
}
