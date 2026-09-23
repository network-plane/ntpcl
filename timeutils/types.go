package timeutils

import (
	"time"

	"github.com/beevik/ntp"
)

// DefaultTimeout is the default network timeout for time fetches.
const DefaultTimeout = 5 * time.Second

// FetchConfig configures time source queries.
type FetchConfig struct {
	Timeout       time.Duration
	HighAccuracy  bool
	SampleCount   int
	SampleSpacing time.Duration
	SampleTimeout time.Duration
	QueryTimeout  time.Duration
}

// DefaultFetchConfig returns sensible defaults.
func DefaultFetchConfig() FetchConfig {
	return FetchConfig{
		Timeout:       DefaultTimeout,
		SampleCount:   10,
		SampleSpacing: 200 * time.Millisecond,
		SampleTimeout: 30 * time.Second,
		QueryTimeout:  3 * time.Second,
	}
}

// TimeResult holds a measured time correction from a source.
type TimeResult struct {
	Offset            time.Duration
	RTT               time.Duration
	Server            string
	NTPResponse       *ntp.Response
	IsUTCSource          bool
	DaytimeNoTimezone    bool
	HighAccuracyStats    *HighAccuracyStats
}

// CorrectedTime returns the current corrected time using the stored offset.
func (r TimeResult) CorrectedTime() time.Time {
	return time.Now().Add(r.Offset)
}

// HighAccuracyStats summarizes multi-sample NTP collection.
type HighAccuracyStats struct {
	SampleCount int
	KeptSamples int
	MedianRTT   time.Duration
	MinRTT      time.Duration
	MaxRTT      time.Duration
	MedianOffset time.Duration
}

// DisplayConfig controls formatted output.
type DisplayConfig struct {
	NoColor bool
}
