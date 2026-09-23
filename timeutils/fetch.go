package timeutils

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	rfc868EpochOffset = 2208988800
	maxDaytimeBanner  = 4096
)

var daytimeLayouts = []string{
	"Mon Jan 2 15:04:05 2006",
	"Mon Jan 02 15:04:05 2006",
	time.RFC3339,
	"Mon Jan 2 15:04:05 MST 2006",
	"Mon Jan 2 15:04:05 GMT 2006",
}

// FetchTimeFromDaytimeProtocol fetches the time from a Daytime Protocol server (RFC 867).
func FetchTimeFromDaytimeProtocol(server string, cfg FetchConfig) (TimeResult, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	_, addr := splitHostPortDefault(server, "13")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	dialer := net.Dialer{Timeout: cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return TimeResult{}, &ErrNetwork{Err: err}
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(cfg.Timeout)); err != nil {
		return TimeResult{}, &ErrNetwork{Err: err}
	}

	start := time.Now()
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return TimeResult{}, &ErrNetwork{Err: err}
	}
	if len(line) > maxDaytimeBanner {
		return TimeResult{}, &ErrInvalidTime{Err: fmt.Errorf("daytime response too large")}
	}
	rtt := time.Since(start)

	serverTime, layout, err := parseDaytimeResponse(line)
	if err != nil {
		return TimeResult{}, &ErrInvalidTime{Err: err}
	}
	noZone := layout == "no-zone"
	if noZone {
		fmt.Println("Warning: daytime response has no timezone; treating timestamp as UTC")
	}

	corrected := serverTime.Add(rtt / 2)
	offset := corrected.Sub(time.Now())

	return TimeResult{
		Offset:            offset,
		RTT:               rtt,
		Server:            server,
		IsUTCSource:       true,
		DaytimeNoTimezone: noZone,
	}, nil
}

// parseDaytimeResponse parses a Daytime Protocol banner.
// Returns the matched layout name, or "no-zone" when parsed without timezone info.
func parseDaytimeResponse(response string) (time.Time, string, error) {
	response = strings.TrimSpace(response)
	if response == "" {
		return time.Time{}, "", fmt.Errorf("empty daytime response")
	}

	for _, layout := range daytimeLayouts {
		if t, err := time.Parse(layout, response); err == nil {
			return t.UTC(), layout, nil
		}
	}

	// RFC 867 allows free-form text; try parsing without zone as UTC.
	if t, err := time.Parse("Mon Jan 2 15:04:05 2006", response); err == nil {
		return t.UTC(), "no-zone", nil
	}

	return time.Time{}, "", fmt.Errorf("unable to parse daytime response: %q", response)
}

// FetchTimeFromTimeProtocol fetches the time from a Time Protocol server (RFC 868).
func FetchTimeFromTimeProtocol(server string, cfg FetchConfig) (TimeResult, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	result, err := fetchTimeProtocolUDP(server, cfg.Timeout)
	if err == nil {
		return result, nil
	}

	tcpResult, tcpErr := fetchTimeProtocolTCP(server, cfg.Timeout)
	if tcpErr == nil {
		return tcpResult, nil
	}

	return TimeResult{}, &ErrNetwork{Err: fmt.Errorf("udp failed (%v); tcp failed (%v)", err, tcpErr)}
}

func fetchTimeProtocolUDP(server string, timeout time.Duration) (TimeResult, error) {
	_, addr := splitHostPortDefault(server, "37")
	start := time.Now()

	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return TimeResult{}, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return TimeResult{}, err
	}

	if _, err := conn.Write([]byte{}); err != nil {
		return TimeResult{}, err
	}

	buffer := make([]byte, 4)
	n, err := io.ReadFull(conn, buffer)
	if err != nil {
		return TimeResult{}, err
	}
	if n != 4 {
		return TimeResult{}, fmt.Errorf("invalid response size %d", n)
	}

	rtt := time.Since(start)
	serverTime, err := rfc868SecondsToTime(binary.BigEndian.Uint32(buffer))
	if err != nil {
		return TimeResult{}, err
	}

	corrected := serverTime.Add(rtt / 2)
	return TimeResult{
		Offset:      corrected.Sub(time.Now()),
		RTT:         rtt,
		Server:      server,
		IsUTCSource: true,
	}, nil
}

func fetchTimeProtocolTCP(server string, timeout time.Duration) (TimeResult, error) {
	_, addr := splitHostPortDefault(server, "37")
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return TimeResult{}, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return TimeResult{}, err
	}

	buffer := make([]byte, 4)
	if _, err := io.ReadFull(conn, buffer); err != nil {
		return TimeResult{}, err
	}

	rtt := time.Since(start)
	serverTime, err := rfc868SecondsToTime(binary.BigEndian.Uint32(buffer))
	if err != nil {
		return TimeResult{}, err
	}

	corrected := serverTime.Add(rtt / 2)
	return TimeResult{
		Offset:      corrected.Sub(time.Now()),
		RTT:         rtt,
		Server:      server,
		IsUTCSource: true,
	}, nil
}

func rfc868SecondsToTime(seconds uint32) (time.Time, error) {
	unixTime := int64(seconds) - rfc868EpochOffset
	if unixTime < 0 {
		return time.Time{}, fmt.Errorf("rfc868 timestamp before unix epoch (value %d; note 2036 rollover)", seconds)
	}
	// Reject timestamps unreasonably far in the future.
	if unixTime > time.Now().Add(24*time.Hour).Unix()+365*24*3600 {
		return time.Time{}, fmt.Errorf("rfc868 timestamp unreasonably far in the future")
	}
	return time.Unix(unixTime, 0).UTC(), nil
}

// FetchTimeFromHTTP fetches the time from an HTTP server's Date header.
func FetchTimeFromHTTP(url string, cfg FetchConfig) (TimeResult, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	client := &http.Client{Timeout: cfg.Timeout}
	start := time.Now()

	resp, err := fetchHTTPTime(client, url)
	if err != nil {
		return TimeResult{}, &ErrNetwork{Err: err}
	}
	defer resp.Body.Close()

	rtt := time.Since(start)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TimeResult{}, &ErrNetwork{Err: fmt.Errorf("http status %d", resp.StatusCode)}
	}

	dateHeader := resp.Header.Get("Date")
	if dateHeader == "" {
		return TimeResult{}, &ErrInvalidTime{Err: fmt.Errorf("no Date header in response")}
	}

	serverTime, err := http.ParseTime(dateHeader)
	if err != nil {
		return TimeResult{}, &ErrInvalidTime{Err: err}
	}

	corrected := serverTime.Add(rtt / 2)
	return TimeResult{
		Offset:      corrected.Sub(time.Now()),
		RTT:         rtt,
		Server:      url,
		IsUTCSource: true,
	}, nil
}

func fetchHTTPTime(client *http.Client, url string) (*http.Response, error) {
	resp, err := client.Head(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented {
		resp.Body.Close()
		return client.Get(url)
	}
	return resp, nil
}
