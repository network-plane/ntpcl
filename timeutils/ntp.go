package timeutils

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/beevik/ntp"
)

func queryNTP(server string, timeout time.Duration) (*ntp.Response, error) {
	response, err := ntp.QueryWithOptions(server, ntp.QueryOptions{Timeout: timeout})
	if err != nil {
		return nil, &ErrNetwork{Err: err}
	}
	if err := response.Validate(); err != nil {
		if response.IsKissOfDeath() {
			return nil, &ErrInvalidTime{Err: fmt.Errorf("kiss-of-death from server (code %q): %w", response.KissCode, err)}
		}
		return nil, &ErrInvalidTime{Err: fmt.Errorf("ntp response invalid (stratum %d, ref %s): %w", response.Stratum, response.ReferenceString(), err)}
	}
	return response, nil
}

// FetchTimeFromNTP fetches time from an NTP server by hostname or address.
func FetchTimeFromNTP(server string, cfg FetchConfig) (TimeResult, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.QueryTimeout == 0 {
		cfg.QueryTimeout = cfg.Timeout
	}

	displayServer := formatServerDisplay(server)

	if cfg.HighAccuracy {
		offset, medianRTT, response, stats, err := gatherHighAccuracyTime(server, cfg)
		if err != nil {
			return TimeResult{}, err
		}
		return TimeResult{
			Offset:            offset,
			RTT:               medianRTT,
			Server:            displayServer,
			NTPResponse:       response,
			HighAccuracyStats: stats,
		}, nil
	}

	response, err := queryNTP(server, cfg.QueryTimeout)
	if err != nil {
		return TimeResult{}, err
	}

	return TimeResult{
		Offset:      response.ClockOffset,
		RTT:         response.RTT,
		Server:      displayServer,
		NTPResponse: response,
	}, nil
}

func gatherHighAccuracyTime(server string, cfg FetchConfig) (time.Duration, time.Duration, *ntp.Response, *HighAccuracyStats, error) {
	if cfg.SampleCount <= 0 {
		cfg.SampleCount = 10
	}
	if cfg.SampleSpacing <= 0 {
		cfg.SampleSpacing = 200 * time.Millisecond
	}
	if cfg.SampleTimeout <= 0 {
		cfg.SampleTimeout = 30 * time.Second
	}
	if cfg.QueryTimeout <= 0 {
		cfg.QueryTimeout = 3 * time.Second
	}

	deadline := time.Now().Add(cfg.SampleTimeout)
	var samples []sampleResult
	var rateLimited bool

	fmt.Printf("High accuracy mode: collecting up to %d samples from %s...\n", cfg.SampleCount, server)

	for i := 0; i < cfg.SampleCount && time.Now().Before(deadline); i++ {
		if i > 0 {
			time.Sleep(cfg.SampleSpacing)
		}

		response, err := ntp.QueryWithOptions(server, ntp.QueryOptions{Timeout: cfg.QueryTimeout})
		if err != nil {
			fmt.Printf("  sample %d/%d failed: %v\n", i+1, cfg.SampleCount, err)
			continue
		}
		if response.IsKissOfDeath() {
			rateLimited = response.KissCode == "RATE"
			fmt.Printf("  sample %d/%d rejected: kiss-of-death (%q)\n", i+1, cfg.SampleCount, response.KissCode)
			if rateLimited {
				time.Sleep(2 * time.Second)
			}
			continue
		}
		if err := response.Validate(); err != nil {
			fmt.Printf("  sample %d/%d rejected: %v\n", i+1, cfg.SampleCount, err)
			continue
		}

		rtt := response.RTT
		if rtt <= 0 {
			rtt = cfg.QueryTimeout
		}
		samples = append(samples, sampleResult{
			offset: response.ClockOffset,
			rtt:    rtt,
			resp:   response,
		})
		fmt.Printf("  sample %d/%d ok (offset %s, rtt %s)\n", i+1, cfg.SampleCount, response.ClockOffset, rtt)
	}

	if len(samples) == 0 {
		return 0, 0, nil, nil, &ErrInvalidTime{Err: errors.New("no valid ntp samples collected")}
	}

	sortSamplesByRTT(samples)

	trimStart := len(samples) / 5
	trimEnd := len(samples) - len(samples)/5
	if trimEnd <= trimStart {
		trimStart = 0
		trimEnd = len(samples)
	}
	kept := samples[trimStart:trimEnd]

	offsets := make([]time.Duration, len(kept))
	rtts := make([]time.Duration, len(kept))
	for i, sample := range kept {
		offsets[i] = sample.offset
		rtts[i] = sample.rtt
	}

	medianOffset := medianDuration(offsets)
	medianRTT := medianDuration(rtts)
	minRTT := rtts[0]
	maxRTT := rtts[len(rtts)-1]
	bestResponse := kept[0].resp

	stats := &HighAccuracyStats{
		SampleCount:  cfg.SampleCount,
		KeptSamples:  len(kept),
		MedianRTT:    medianRTT,
		MinRTT:       minRTT,
		MaxRTT:       maxRTT,
		MedianOffset: medianOffset,
	}

	fmt.Printf("Kept %d/%d samples; median offset %s, median RTT %s\n", len(kept), len(samples), medianOffset, medianRTT)

	return medianOffset, medianRTT, bestResponse, stats, nil
}

type sampleResult struct {
	offset time.Duration
	rtt    time.Duration
	resp   *ntp.Response
}

func sortSamplesByRTT(samples []sampleResult) {
	for i := 0; i < len(samples); i++ {
		for j := i + 1; j < len(samples); j++ {
			if samples[j].rtt < samples[i].rtt {
				samples[i], samples[j] = samples[j], samples[i]
			}
		}
	}
}

func formatServerDisplay(server string) string {
	host := server
	if h, _, err := net.SplitHostPort(server); err == nil {
		host = h
	}
	if net.ParseIP(host) != nil {
		return server
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return server
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return fmt.Sprintf("%s (%s)", server, v4.String())
		}
	}
	return fmt.Sprintf("%s (%s)", server, ips[0].String())
}

func splitHostPortDefault(server string, defaultPort string) (network, address string) {
	if strings.Contains(server, "://") {
		return "", server
	}
	host, port, err := net.SplitHostPort(server)
	if err != nil {
		if strings.Contains(err.Error(), "missing port") {
			return "", net.JoinHostPort(server, defaultPort)
		}
		return "", net.JoinHostPort(server, defaultPort)
	}
	if host == "" {
		return "", net.JoinHostPort(server, defaultPort)
	}
	return "", net.JoinHostPort(host, port)
}
