package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/earentir/ntpcl/timeutils"
	"github.com/spf13/cobra"
)

const defaultNTPServer = "europe.pool.ntp.org"

var appVersion = "0.4.23"

const (
	exitOK          = 0
	exitGeneral     = 1
	exitUsage       = 2
	exitNetwork     = 3
	exitInvalidTime = 4
	exitPermission  = 5
)

type commandOptions struct {
	set            bool
	useSystemTools bool
	highAccuracy   bool
	dryRun         bool
	force          bool
	quiet          bool
	noColor        bool
	timeIsUTC      bool
	timeout        time.Duration
	maxDelta       time.Duration
	format         string
}

func main() {
	var opts commandOptions

	rootCmd := &cobra.Command{
		Use:     "ntpcl",
		Short:   "A simple time client to fetch and optionally set system time",
		Long:    "A simple time client to fetch and optionally set system time. It can be used to query an NTP server, HTTP server, Daytime Protocol server, or Time Protocol server for the current time and set the system time to the retrieved time.\nhttps://github.com/earentir/ntpcl",
		Version: appVersion,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithSource(opts, "", "", "", defaultNTPServer, "")
		},
	}

	addCommonFlags(rootCmd, &opts)
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	rootCmd.SetVersionTemplate("{{.CommandPath}} v{{.Version}}\n")

	ntpCmd := &cobra.Command{
		Use:   "ntp SERVER",
		Short: "Fetch time from an NTP server",
		Args:  requireSingleArg("ntp", "an NTP server address"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithSource(opts, "", "", "", args[0], "")
		},
	}

	httpCmd := &cobra.Command{
		Use:   "http URL",
		Short: "Fetch time from an HTTP server's Date header",
		Args:  requireSingleArg("http", "a URL"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithSource(opts, args[0], "", "", "", "")
		},
	}

	daytimeCmd := &cobra.Command{
		Use:   "daytime SERVER",
		Short: "Fetch time from a Daytime Protocol server",
		Args:  requireSingleArg("daytime", "a server address"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithSource(opts, "", args[0], "", "", "")
		},
	}

	timeCmd := &cobra.Command{
		Use:   "time SERVER",
		Short: "Fetch time from a Time Protocol (RFC 868) server",
		Args:  requireSingleArg("time", "a server address"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithSource(opts, "", "", args[0], "", "")
		},
	}

	windowsCmd := &cobra.Command{
		Use:   "windows-time SERVER",
		Short: "Query a Windows Time host over NTP (unsigned; not MS-SNTP authenticated)",
		Args:  requireSingleArg("windows-time", "a server address"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithSource(opts, "", "", "", "", args[0])
		},
	}

	rootCmd.AddCommand(ntpCmd, httpCmd, daytimeCmd, timeCmd, windowsCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCodeFor(err))
	}
}

func addCommonFlags(cmd *cobra.Command, opts *commandOptions) {
	cmd.PersistentFlags().BoolVar(&opts.set, "set", false, "Set the system time")
	cmd.PersistentFlags().BoolVar(&opts.useSystemTools, "system-tools", false, "Use system commands to set time instead of system calls (only with --set)")
	cmd.PersistentFlags().BoolVar(&opts.highAccuracy, "high-accuracy", false, "Use high accuracy mode (only with NTP sources)")
	cmd.PersistentFlags().BoolVar(&opts.dryRun, "dry-run", false, "Show what would be set without changing the system clock")
	cmd.PersistentFlags().BoolVar(&opts.force, "force", false, "Allow large clock steps and high RTT sources")
	cmd.PersistentFlags().BoolVar(&opts.quiet, "quiet", false, "Print only the corrected time")
	cmd.PersistentFlags().BoolVar(&opts.noColor, "no-color", false, "Disable colored output")
	cmd.PersistentFlags().BoolVar(&opts.timeIsUTC, "time-is-utc", false, "Treat daytime responses without timezone as UTC when setting time")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", timeutils.DefaultTimeout, "Network timeout for time fetches")
	cmd.PersistentFlags().DurationVar(&opts.maxDelta, "max-delta", 1*time.Second, "Maximum allowed clock step when using --set")
	cmd.PersistentFlags().StringVar(&opts.format, "format", "table", "Output format: table, json, unix, rfc3339")
}

func requireSingleArg(commandName, argDescription string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errorWithCommandHelp(cmd, fmt.Sprintf("missing %s: provide %s when using the %s command", argDescription, argDescription, commandName))
		}
		if len(args) > 1 {
			return errorWithCommandHelp(cmd, fmt.Sprintf("the %s command accepts only one argument (%s)", commandName, argDescription))
		}
		return nil
	}
}

func errorWithCommandHelp(cmd *cobra.Command, message string) error {
	var help bytes.Buffer
	originalOut := cmd.OutOrStdout()
	originalErr := cmd.ErrOrStderr()
	cmd.SetOut(&help)
	cmd.SetErr(&help)
	_ = cmd.Help()
	cmd.SetOut(originalOut)
	cmd.SetErr(originalErr)
	return fmt.Errorf("%s\n\n%s", message, help.String())
}

func runWithSource(opts commandOptions, httpURL, daytimeServer, timeProtocolServer, ntpServer, windowsTimeServer string) error {
	timeutils.InitColors(opts.noColor)

	if opts.useSystemTools && !opts.set {
		fmt.Fprintln(os.Stderr, "warning: --system-tools has no effect without --set")
	}

	if opts.highAccuracy && ntpServer == "" && windowsTimeServer == "" {
		return fmt.Errorf("--high-accuracy can only be used with NTP sources")
	}

	maxDelta := opts.maxDelta
	if opts.highAccuracy && maxDelta == 1*time.Second {
		maxDelta = 100 * time.Millisecond
	}

	cfg := timeutils.DefaultFetchConfig()
	cfg.Timeout = opts.timeout
	cfg.HighAccuracy = opts.highAccuracy

	method, result, err := fetchTime(httpURL, daytimeServer, timeProtocolServer, ntpServer, windowsTimeServer, cfg)
	if err != nil {
		return fmt.Errorf("failed to fetch time: %w", err)
	}

	displayCfg := timeutils.DisplayConfig{NoColor: opts.noColor}

	if opts.format == "json" {
		if err := timeutils.WriteJSON(os.Stdout, method, result); err != nil {
			return err
		}
	} else if opts.format == "unix" {
		fmt.Println(result.CorrectedTime().Unix())
	} else if opts.format == "rfc3339" {
		fmt.Println(result.CorrectedTime().Format(time.RFC3339Nano))
	} else if opts.quiet {
		fmt.Println(result.CorrectedTime().Format(time.RFC3339Nano))
	} else {
		timeutils.DisplayTimeInfo(method, result, displayCfg)
	}

	if method == "HTTP" || method == "Daytime" {
		fmt.Fprintln(os.Stderr, "warning: time from "+method+" is unauthenticated and may be coarse")
	}

	if result.DaytimeNoTimezone && opts.set && !opts.timeIsUTC && !opts.dryRun {
		return fmt.Errorf("daytime response has no timezone; pass --time-is-utc to allow --set")
	}

	if !opts.set {
		if opts.dryRun {
			fmt.Println("dry-run: system clock would be set to", result.CorrectedTime().Format(time.RFC3339Nano))
		}
		return nil
	}

	if opts.dryRun {
		fmt.Println("dry-run: system clock would be set to", result.CorrectedTime().Format(time.RFC3339Nano))
		return nil
	}

	if !opts.force && result.RTT > time.Second && result.IsUTCSource {
		return fmt.Errorf("round-trip time %s is too high for --set; use --force to override", result.RTT)
	}

	delta := result.Offset
	if delta < 0 {
		delta = -delta
	}
	if !opts.force && delta > maxDelta {
		return fmt.Errorf("clock step %s exceeds max-delta %s; use --force or raise --max-delta", result.Offset, maxDelta)
	}

	setTime := time.Now().Add(result.Offset)
	if err := timeutils.SetSystemTimeWrapper(setTime, opts.useSystemTools); err != nil {
		return fmt.Errorf("failed to set system time: %w", err)
	}

	if !opts.quiet && opts.format == "table" {
		fmt.Println("System time updated successfully")
		printNewTimeInfo(result, displayCfg)
	}

	return nil
}

func fetchTime(httpURL, daytimeServer, timeProtocolServer, ntpServer, windowsTimeServer string, cfg timeutils.FetchConfig) (string, timeutils.TimeResult, error) {
	switch {
	case httpURL != "":
		result, err := timeutils.FetchTimeFromHTTP(httpURL, cfg)
		return "HTTP", result, err
	case daytimeServer != "":
		result, err := timeutils.FetchTimeFromDaytimeProtocol(daytimeServer, cfg)
		return "Daytime", result, err
	case timeProtocolServer != "":
		result, err := timeutils.FetchTimeFromTimeProtocol(timeProtocolServer, cfg)
		return "Time Protocol", result, err
	case ntpServer != "":
		result, err := timeutils.FetchTimeFromNTP(ntpServer, cfg)
		return "NTP", result, err
	case windowsTimeServer != "":
		result, err := timeutils.FetchTimeFromNTP(windowsTimeServer, cfg)
		return "NTP", result, err
	default:
		result, err := timeutils.FetchTimeFromNTP(defaultNTPServer, cfg)
		return "NTP", result, err
	}
}

func printNewTimeInfo(result timeutils.TimeResult, cfg timeutils.DisplayConfig) {
	newLocalTime := time.Now()
	target := newLocalTime.Add(result.Offset)
	timeDiff := newLocalTime.Sub(target)
	fmt.Print(timeutils.FormattedOutput("Local Time Update", target, newLocalTime, timeDiff, 0, "", nil, nil, cfg))
}

func exitCodeFor(err error) int {
	var netErr *timeutils.ErrNetwork
	if errors.As(err, &netErr) {
		return exitNetwork
	}
	var invalidErr *timeutils.ErrInvalidTime
	if errors.As(err, &invalidErr) {
		return exitInvalidTime
	}
	var permErr *timeutils.ErrPermission
	if errors.As(err, &permErr) {
		return exitPermission
	}

	msg := err.Error()
	if strings.Contains(msg, "missing ") || strings.Contains(msg, "accepts only one argument") {
		return exitUsage
	}
	if strings.Contains(msg, "exceeds max-delta") || strings.Contains(msg, "kiss-of-death") {
		return exitInvalidTime
	}
	if strings.Contains(msg, "failed to fetch time") {
		return exitNetwork
	}
	if strings.Contains(msg, "failed to set system time") {
		return exitPermission
	}

	return exitGeneral
}
