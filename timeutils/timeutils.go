// Package timeutils provides utility functions for fetching and setting the system time using various time protocols.
package timeutils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/beevik/ntp"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

type sampleResult struct {
	offset    time.Duration
	rtt       time.Duration
	timestamp time.Time
}

func medianDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

// FetchTimeFromDaytimeProtocol fetches the time from a server using the Daytime Protocol (RFC 867).
func FetchTimeFromDaytimeProtocol(server string) (time.Time, time.Duration, error) {
	start := time.Now()
	conn, err := net.Dial("tcp", net.JoinHostPort(server, "13"))
	if err != nil {
		return time.Time{}, 0, err
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return time.Time{}, 0, err
	}

	rtt := time.Since(start)

	serverTime, err := parseDaytimeResponse(response)
	if err != nil {
		return time.Time{}, 0, err
	}

	return serverTime, rtt, nil
}

// parseDaytimeResponse parses the response from the Daytime Protocol to extract the time.
func parseDaytimeResponse(response string) (time.Time, error) {
	response = strings.TrimSpace(response)
	serverTime, err := time.Parse("Mon Jan 2 15:04:05 2006", response)
	if err != nil {
		return time.Time{}, err
	}
	return serverTime, nil
}

// FetchTimeFromTimeProtocol fetches the time from a server using the Time Protocol (RFC 868).
func FetchTimeFromTimeProtocol(server string) (time.Time, time.Duration, error) {
	start := time.Now()
	conn, err := net.Dial("udp", net.JoinHostPort(server, "37"))
	if err != nil {
		return time.Time{}, 0, err
	}
	defer conn.Close()

	buffer := make([]byte, 4)
	n, err := conn.Read(buffer)
	if err != nil {
		return time.Time{}, 0, err
	}
	if n != 4 {
		return time.Time{}, 0, fmt.Errorf("invalid response size")
	}

	rtt := time.Since(start)

	seconds := binary.BigEndian.Uint32(buffer)
	unixTime := int64(seconds) - 2208988800 // Convert to Unix time (seconds since 1970)

	serverTime := time.Unix(unixTime, 0).UTC()

	return serverTime, rtt, nil
}

// FetchTimeFromHTTP fetches the time from an HTTP server's Date header.
func FetchTimeFromHTTP(url string) (time.Time, time.Duration, error) {
	start := time.Now()
	resp, err := http.Head(url)
	if err != nil {
		return time.Time{}, 0, err
	}
	rtt := time.Since(start)
	defer resp.Body.Close()

	dateHeader := resp.Header.Get("Date")
	if dateHeader == "" {
		return time.Time{}, 0, fmt.Errorf("no Date header found in response")
	}

	serverTime, err := time.Parse(time.RFC1123, dateHeader)
	if err != nil {
		return time.Time{}, 0, err
	}

	return serverTime, rtt, nil
}

// FetchTimeFromNTP fetches the time from an NTP server.
func FetchTimeFromNTP(ntpServer, windowsTimeServer string, highAccuracy bool) (time.Time, time.Duration, *ntp.Response, string, error) {
	var serverToUse string
	if windowsTimeServer != "" {
		serverToUse = windowsTimeServer
	} else {
		serverToUse = ntpServer
	}

	// Check if serverToUse is an IP address
	if net.ParseIP(serverToUse) == nil {
		// If it's not an IP address, resolve the hostname
		ip, err := GetServerIP(serverToUse)
		if err != nil {
			return time.Time{}, 0, nil, "", fmt.Errorf("failed to get IP address for server: %v", err)
		}
		serverToUse = ip
	}

	if highAccuracy {
		serverTime, err := GatherHighAccuracyTime(serverToUse)
		if err != nil {
			return time.Time{}, 0, nil, "", err
		}
		// For high accuracy mode, we don't have a single NTP response to return
		return serverTime, 0, nil, serverToUse, nil
	}

	response, err := ntp.Query(serverToUse)
	if err != nil {
		return time.Time{}, 0, nil, "", err
	}

	serverTime := time.Now().Add(response.ClockOffset)

	return serverTime, response.RTT, response, serverToUse, nil
}

// GatherHighAccuracyTime gathers multiple samples to get a high accuracy time.
func GatherHighAccuracyTime(ntpServerToUse string) (time.Time, error) {
	fmt.Println("High accuracy mode enabled. Gathering multiple samples in parallel...")

	const (
		sampleCount    = 10
		timeoutSeconds = 5
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeoutSeconds*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	results := make(chan sampleResult, sampleCount)

	for i := 0; i < sampleCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					start := time.Now()
					resp, err := ntp.Query(ntpServerToUse)
					if err != nil {
						fmt.Printf("Sample query failed: %v. Retrying...\n", err)
						time.Sleep(100 * time.Millisecond)
						continue
					}
					rtt := resp.RTT
					if rtt <= 0 {
						rtt = time.Since(start)
					}
					results <- sampleResult{
						offset:    resp.ClockOffset,
						rtt:       rtt,
						timestamp: start.Add(rtt / 2), // Approximate time when server was queried
					}
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var samples []sampleResult
	for result := range results {
		samples = append(samples, result)
	}

	if len(samples) < sampleCount {
		return time.Time{}, fmt.Errorf("failed to gather enough samples, got %d out of %d", len(samples), sampleCount)
	}

	// Sort samples by RTT
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].rtt < samples[j].rtt
	})

	// Use the median 60% of samples
	validSamples := samples[sampleCount/5 : 4*sampleCount/5]

	offsets := make([]time.Duration, len(validSamples))
	rtts := make([]time.Duration, len(validSamples))
	var latestTimestamp time.Time

	for i, sample := range validSamples {
		offsets[i] = sample.offset
		rtts[i] = sample.rtt
		if sample.timestamp.After(latestTimestamp) {
			latestTimestamp = sample.timestamp
		}
	}

	medianOffset := medianDuration(offsets)
	medianRTT := medianDuration(rtts)

	// Calculate the time elapsed since the latest sample
	elapsedSinceLastSample := time.Since(latestTimestamp)

	// Adjust the final time calculation
	adjustedTime := time.Now().Add(medianOffset)

	fmt.Printf("Median offset: %v\n", medianOffset)
	fmt.Printf("Median RTT: %v\n", medianRTT)
	fmt.Printf("Elapsed since last sample: %v\n", elapsedSinceLastSample)
	fmt.Printf("Adjusted time: %v\n", adjustedTime)

	return adjustedTime, nil
}

// SetSystemTimeWrapper decides whether to use system calls or system commands.
func SetSystemTimeWrapper(t time.Time, useSystemTools bool) error {
	if useSystemTools {
		return SetSystemTimeWithCommand(t)
	}
	return SetSystemTime(t)
}

// SetSystemTimeWithCommand sets the system time using system commands.
func SetSystemTimeWithCommand(t time.Time) error {
	var cmd *exec.Cmd
	formattedTime := t.Format("2006-01-02 15:04:05.000000000")

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/C", "date", formattedTime[:10])
		if err := cmd.Run(); err != nil {
			return err
		}
		cmd = exec.Command("cmd", "/C", "time", formattedTime[11:])
	case "linux":
		cmd = exec.Command("sudo", "date", "-s", formattedTime)
	case "darwin":
		cmd = exec.Command("sudo", "date", "-u", formattedTime)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Run()
}

// GetServerIP resolves the IP address of the server.
func GetServerIP(server string) (string, error) {
	ips, err := net.LookupIP(server)
	if err != nil {
		return "", err
	}
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String(), nil
		}
	}
	return "", fmt.Errorf("no IPv4 address found for server %s", server)
}

// QueryNTPTime queries the NTP server for the current time.
func QueryNTPTime(server string) (*ntp.Response, time.Duration, error) {
	start := time.Now()
	response, err := ntp.Query(server)
	if err != nil {
		return nil, 0, err
	}
	rtt := time.Since(start)
	return response, rtt, nil
}

// QuerySNTPTime queries the SNTP server for the current time.
func QuerySNTPTime(server string) (*ntp.Response, time.Duration, error) {
	return QueryNTPTime(server)
}

// DisplayTimeInfo displays the fetched time and round trip time
func DisplayTimeInfo(method string, serverTime time.Time, roundTripTime time.Duration, server string, ntpResponse *ntp.Response) {
	localTime := time.Now()
	timeDiff := serverTime.Sub(localTime)

	fmt.Print(FormattedOutput(method, serverTime, localTime, timeDiff, roundTripTime, server, ntpResponse))
}

// PrintNTPDetails prints the details of the NTP response
func PrintNTPDetails(method string, serverTime time.Time, rtt time.Duration, server, serverIP string, response *ntp.Response) {
	localTime := time.Now()
	timeDiff := serverTime.Sub(localTime)

	fmt.Print(FormattedOutput(method, serverTime, localTime, timeDiff, rtt, fmt.Sprintf("%s (%s)", server, serverIP), response))
}

// FormattedOutput generates a formatted string for displaying time information
func FormattedOutput(method string, serverTime, localTime time.Time, timeDiff, rtt time.Duration, server string, ntpResponse *ntp.Response) string {
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
		addRow("Precision", fmt.Sprintf("%d", ntpResponse.Precision))
		addRow("Root Delay", ntpResponse.RootDelay.String())
		addRow("Root Dispersion", ntpResponse.RootDispersion.String())
		addColoredRow("Clock Offset", ntpResponse.ClockOffset.String(), ntpResponse.ClockOffset)
		addRow("Poll Interval", ntpResponse.Poll.String())
	}

	table.Render()
	return buf.String()
}
