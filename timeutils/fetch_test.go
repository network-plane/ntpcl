package timeutils

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestParseDaytimeResponse(t *testing.T) {
	tm, layout, err := parseDaytimeResponse("Wed Sep 23 12:00:00 2025\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if layout == "" {
		t.Fatal("expected layout name")
	}
	if tm.Year() != 2025 {
		t.Fatalf("expected year 2025, got %d", tm.Year())
	}
}

func TestParseDaytimeResponseRFC3339(t *testing.T) {
	tm, _, err := parseDaytimeResponse("2025-09-23T12:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.UTC().Hour() != 12 {
		t.Fatalf("expected hour 12, got %d", tm.Hour())
	}
}

func TestRFC868SecondsToTime(t *testing.T) {
	// 2025-01-01 00:00:00 UTC
	unix := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	seconds := uint32(unix + rfc868EpochOffset)

	tm, err := rfc868SecondsToTime(seconds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.UTC().Year() != 2025 {
		t.Fatalf("expected 2025, got %d", tm.Year())
	}
}

func TestRFC868SecondsBeforeEpoch(t *testing.T) {
	_, err := rfc868SecondsToTime(0)
	if err == nil {
		t.Fatal("expected error for pre-epoch value")
	}
}

func TestFetchTimeProtocolUDPRequiresRequest(t *testing.T) {
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := ln.ReadFrom(buf)
			if err != nil {
				return
			}
			if n == 0 {
				resp := make([]byte, 4)
				unix := time.Now().UTC().Unix()
				binary.BigEndian.PutUint32(resp, uint32(unix+rfc868EpochOffset))
				_, _ = ln.WriteTo(resp, addr)
				return
			}
		}
	}()

	host, port, err := net.SplitHostPort(ln.LocalAddr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	result, err := fetchTimeProtocolUDP(net.JoinHostPort(host, port), 2*time.Second)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if result.Offset == 0 && result.RTT == 0 {
		t.Fatal("expected non-zero result")
	}
}

func TestMedianDuration(t *testing.T) {
	got := medianDuration([]time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond})
	if got != 20*time.Millisecond {
		t.Fatalf("expected 20ms, got %s", got)
	}
}
