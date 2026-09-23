package timeutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/beevik/ntp"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// InitColors configures colored output based on flags and environment.
func InitColors(noColor bool) {
	color.NoColor = noColor || os.Getenv("NO_COLOR") != "" || !isatty.IsTerminal(os.Stdout.Fd())
}

// DisplayTimeInfo displays fetched time information.
func DisplayTimeInfo(method string, result TimeResult, cfg DisplayConfig) {
	localTime := time.Now()
	serverTime := result.CorrectedTime()
	timeDiff := serverTime.Sub(localTime)
	fmt.Print(FormattedOutput(method, serverTime, localTime, timeDiff, result.RTT, result.Server, result.NTPResponse, result.HighAccuracyStats, cfg))
}

// JSONHighAccuracyStats is the JSON representation of high-accuracy sampling.
type JSONHighAccuracyStats struct {
	SampleCount  int    `json:"sample_count"`
	KeptSamples  int    `json:"kept_samples"`
	MedianRTT    string `json:"median_rtt"`
	MinRTT       string `json:"min_rtt"`
	MaxRTT       string `json:"max_rtt"`
	MedianOffset string `json:"median_offset"`
}

// JSONOutput is the machine-readable result format.
type JSONOutput struct {
	Method         string                 `json:"method"`
	Server         string                 `json:"server"`
	ServerTime     string                 `json:"server_time"`
	LocalTime      string                 `json:"local_time"`
	Offset         string                 `json:"offset"`
	RTT            string                 `json:"rtt"`
	Stratum        *uint8                 `json:"stratum,omitempty"`
	Valid          bool                   `json:"valid"`
	HighAccuracy   *JSONHighAccuracyStats `json:"high_accuracy,omitempty"`
}

// WriteJSON writes the result as JSON.
func WriteJSON(w io.Writer, method string, result TimeResult) error {
	localTime := time.Now()
	serverTime := result.CorrectedTime()
	out := JSONOutput{
		Method:     method,
		Server:     result.Server,
		ServerTime: serverTime.Format(time.RFC3339Nano),
		LocalTime:  localTime.Format(time.RFC3339Nano),
		Offset:     serverTime.Sub(localTime).String(),
		RTT:        result.RTT.String(),
		Valid:      true,
	}
	if result.HighAccuracyStats != nil {
		ha := result.HighAccuracyStats
		out.HighAccuracy = &JSONHighAccuracyStats{
			SampleCount:  ha.SampleCount,
			KeptSamples:  ha.KeptSamples,
			MedianRTT:    ha.MedianRTT.String(),
			MinRTT:       ha.MinRTT.String(),
			MaxRTT:       ha.MaxRTT.String(),
			MedianOffset: ha.MedianOffset.String(),
		}
	}
	if result.NTPResponse != nil {
		out.Stratum = &result.NTPResponse.Stratum
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// FormattedOutput generates a formatted table for time information.
func FormattedOutput(method string, serverTime, localTime time.Time, timeDiff, rtt time.Duration, server string, ntpResponse *ntp.Response, haStats *HighAccuracyStats, cfg DisplayConfig) string {
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	unicodeInterior := tw.Rendition{
		Borders: tw.Border{
			Left:   tw.Off,
			Right:  tw.Off,
			Top:    tw.Off,
			Bottom: tw.Off,
		},
		Symbols: tw.NewSymbols(tw.StyleLight),
		Settings: tw.Settings{
			Lines: tw.Lines{
				ShowTop:        tw.Off,
				ShowBottom:     tw.Off,
				ShowHeaderLine: tw.On,
				ShowFooterLine: tw.Off,
			},
			Separators: tw.Separators{
				ShowHeader:     tw.Off,
				ShowFooter:     tw.Off,
				BetweenRows:    tw.Off,
				BetweenColumns: tw.On,
			},
		},
	}

	table.Options(
		tablewriter.WithRowAlignment(tw.AlignLeft),
		tablewriter.WithHeaderAlignment(tw.AlignCenter),
		tablewriter.WithRendition(unicodeInterior),
	)

	table.Header("Property", "Value")

	addRow := func(property, value string) {
		table.Append([]string{property, value})
	}

	addColoredRow := func(property, value string, duration time.Duration) {
		if cfg.NoColor {
			addRow(property, value)
			return
		}
		coloredValue := value
		switch {
		case duration.Abs() < 250*time.Millisecond:
			coloredValue = color.GreenString(value)
		case duration.Abs() < 1*time.Second:
			coloredValue = color.YellowString(value)
		default:
			coloredValue = color.RedString(value)
		}
		table.Append([]string{property, coloredValue})
	}

	addRow("Method", method)
	addRow("Server Time", serverTime.Format(time.RFC3339Nano))
	addRow("Local Time", localTime.Format(time.RFC3339Nano))
	addColoredRow("Time Difference", timeDiff.String(), timeDiff)
	addRow("Round Trip Time", rtt.String())
	if server != "" {
		addRow("Server", server)
	}

	if ntpResponse != nil {
		addRow("Stratum", fmt.Sprintf("%d", ntpResponse.Stratum))
		addRow("Version", fmt.Sprintf("%d", ntpResponse.Version))
		addRow("Precision", ntpResponse.Precision.String())
		addRow("Root Delay", ntpResponse.RootDelay.String())
		addRow("Root Dispersion", ntpResponse.RootDispersion.String())
		addRow("Root Distance", ntpResponse.RootDistance.String())
		addColoredRow("Clock Offset", ntpResponse.ClockOffset.String(), ntpResponse.ClockOffset)
		addRow("Poll Interval", ntpResponse.Poll.String())
		addRow("Leap Indicator", fmt.Sprintf("%d", ntpResponse.Leap))
		addRow("Reference ID", ntpResponse.ReferenceString())
		addRow("Reference Time", ntpResponse.ReferenceTime.Format(time.RFC3339Nano))
		if ntpResponse.KissCode != "" {
			addRow("Kiss Code", ntpResponse.KissCode)
		}
	}

	if haStats != nil {
		addRow("Samples Kept", fmt.Sprintf("%d of %d", haStats.KeptSamples, haStats.SampleCount))
		addRow("Median RTT", haStats.MedianRTT.String())
		addRow("Min RTT", haStats.MinRTT.String())
		addRow("Max RTT", haStats.MaxRTT.String())
		addRow("Median Offset", haStats.MedianOffset.String())
	}

	table.Render()
	return buf.String()
}

func medianDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}
