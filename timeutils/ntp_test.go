package timeutils

import (
	"testing"

	"github.com/beevik/ntp"
)

func TestQueryNTPValidationRejectsKissOfDeath(t *testing.T) {
	// We cannot easily mock UDP NTP here without a full server; test validation path via response object.
	resp := &ntp.Response{
		Stratum: 0,
		KissCode: "RATE",
	}
	if !resp.IsKissOfDeath() {
		t.Fatal("expected kiss of death")
	}
	if err := resp.Validate(); err == nil {
		t.Fatal("expected validation error for kiss-of-death")
	}
}

func TestDefaultFetchConfig(t *testing.T) {
	cfg := DefaultFetchConfig()
	if cfg.Timeout != DefaultTimeout {
		t.Fatalf("expected default timeout %s, got %s", DefaultTimeout, cfg.Timeout)
	}
	if cfg.SampleCount != 10 {
		t.Fatalf("expected 10 samples, got %d", cfg.SampleCount)
	}
}
