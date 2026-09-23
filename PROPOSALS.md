# ntpcl review and proposals

Review of the tree as of version `0.4.23` (`main.go`, `timeutils/`). This is a proposal list, not a changelog. Items marked **DONE** have been implemented.

## What is in good shape

- NTP sync uses `ClockOffset` (`time.Now().Add(response.ClockOffset)`), which is what `github.com/beevik/ntp` documents for synchronization. `Response.Time` is correctly left unused for that purpose.
- Platform time-setting is split into `timeutils_*_{linux,darwin,windows}.go` instead of a pile of `runtime.GOOS` branches in the syscall path.
- The Cobra layout (default NTP query, plus `ntp`, `http`, `daytime`, `time`, `windows-time`) is clearer than the old flag-only interface the README still describes.
- High-accuracy mode already has the right shape: several samples, drop the worst RTTs, take a median offset.

## P0 — wrong clock or a hang

### 1. RFC 868 client never sends a request — **DONE**

`FetchTimeFromTimeProtocol` dials UDP/37 and blocks in `Read`. RFC 868 requires the client to send an empty datagram first; the server replies with 4 bytes only after that. There is also no read deadline, so a silent server hangs the process.

`Query` in `github.com/beevik/ntp` does send a packet. This path does not.

**Proposal**

- Write one empty datagram, then read exactly 4 bytes.
- Set a deadline (shared timeout flag, default a few seconds).
- Offer TCP/37 as well: on TCP the server sends the 4 bytes on connect, which is the other half of RFC 868.
- Document the 2036 rollover. The protocol is a `uint32` of seconds since 1900-01-01, which wraps on 2036-02-07. Reject values that cannot be converted, or stop advertising this source as the date approaches.

**Implemented:** `timeutils/fetch.go` — UDP sends empty datagram, TCP fallback, `--timeout`, 2036/pre-epoch rejection, regression test.

### 2. Daytime and HTTP can hang, and HTTP time is only trusted if the header parses one layout — **DONE**

`FetchTimeFromDaytimeProtocol` uses `net.Dial` plus `ReadString('\n')` with no deadline. `FetchTimeFromHTTP` uses `http.Head` on `http.DefaultClient`, which has no timeout.

HTTP dates are parsed only with `time.RFC1123`. Servers still send RFC 850 and ANSI C `asctime`. `net/http.ParseTime` already accepts all three IMF historic forms. A missing or odd `Date` fails the command; a 500 from a CDN still succeeds because the status code is ignored.

**Proposal**

- One timeout value, default 5s, applied to dial, read, and the HTTP client. Pass it through `http.Client{Timeout: ...}` and `context`.
- Parse HTTP dates with `http.ParseTime`.
- Treat non-2xx as an error, and fall back from `HEAD` to `GET` on `405`/`501`. Some servers reject `HEAD`.
- Read the daytime banner until newline or EOF, with a size cap. RFC 867 does not define one layout. Try a short list (`Mon Jan 2 15:04:05 2006`, `Mon Jan 02 15:04:05 2006`, RFC 3339, layouts with `MST`/`GMT`) and say which one matched. If the banner has no zone, do not assume UTC silently: print the assumption and refuse `--set` unless the user passes `--time-is-utc` or similar.

**Implemented:** `timeutils/fetch.go`, `--timeout`, `--time-is-utc`, multiple daytime layouts, HTTP status checks.

### 3. Kiss-of-death and other unsync responses are applied as real time — **DONE**

`ntp.Query` returns a `Response` and a nil error for stratum 0. Validity is a separate call, `Response.Validate()`, and nothing in this repo calls it. A pool that answers `RATE` (common if high-accuracy mode fires 10 queries at once) still produces a `ClockOffset`. `--set` will step the system clock to it.

**Proposal**

- After every query, call `Validate()`. On failure, print stratum, `KissCode`, and `ReferenceString()`, and do not set the clock.
- In high-accuracy mode, drop invalid samples and back off on `RATE` instead of retrying every 100ms.

**Implemented:** `timeutils/ntp.go` — `queryNTP()` validates every response; high-accuracy mode rejects kiss-of-death and backs off on `RATE`.

### 4. `--set` steps to a timestamp that is already stale — **DONE**

The NTP offset is applied once, during fetch, and the resulting `time.Time` is what gets passed to `SetSystemTime`. Display, argument parsing, and process scheduling all happen before the syscall, so the clock is set to "correct time a few milliseconds ago". High-accuracy mode makes this worse: it also returns round-trip `0` and throws away `medianRTT`, and it logs `elapsedSinceLastSample` without using it.

Non-NTP sources are worse. HTTP, daytime, and time-protocol results are the server's stamp at the start of the exchange. One-way delay (about half the RTT) is never removed. HTTP `Date` is only 1 second resolution on top of that.

**Proposal**

- Keep `offset` and `measuredAt`, and set `time.Now().Add(offset)` immediately before the syscall.
- For non-NTP sources, subtract `rtt/2`, and refuse `--set` when RTT is large (for example over 1s) unless `--force` is set.
- Return the median RTT and a representative `*ntp.Response` from high-accuracy mode so the table is not a row of zeros.
- Apply the offset at the end. Delete the unused elapsed-time adjustment, or actually use it. Do not print both.

**Implemented:** `TimeResult.Offset` stored; `time.Now().Add(offset)` at set time; RTT/2 correction for non-NTP; high-accuracy returns median RTT and stats.

### 5. No guard before stepping the clock — **DONE**

Any successful response can move the clock by hours. That breaks TLS, logs, cron, and build tools. A hostile or simply wrong server is enough. There is no dry run.

**Proposal**

- Default: refuse to step more than a threshold (start at 1s, or 100ms in high-accuracy mode). Print the delta and the command to override, `--max-delta 5s` or `--force`.
- `--dry-run` prints the table and the exact set operation, then exits 0 without changing the clock.
- Warn when the source is HTTP or daytime, because those are unauthenticated and coarse.

**Implemented:** `--max-delta`, `--dry-run`, `--force`, warnings for HTTP/Daytime.

### 6. System-command setter is wrong on macOS and Windows — **DONE**

`SetSystemTimeWithCommand`:

- macOS runs `sudo date -u 2006-01-02 15:04:05.000000000`. The system `date` expects `[[[mm]dd]HH]MM[[cc]yy][.ss]` unless `-f` is given, so this fails.
- Windows runs `cmd /C date` with `YYYY-MM-DD`, then `time` with a 9-digit fractional second. `date` is locale-specific (`MM-DD-YYYY` on many US installs) and the two calls are not atomic: a failed `time` leaves the date already changed.
- Linux `date -s` interprets the formatted wall time in the local zone. NTP results are local (`time.Now().Add`), so they happen to work. HTTP and daytime results are UTC, so `date -s` shifts them by the timezone offset.

The syscall path is the one to keep. The command path needs a rewrite per OS, or it should be removed until it is tested.

**Proposal**

- Format an explicit UTC instant and pass a format each OS actually accepts (`date -u -s` on GNU, `date -u -f` on macOS, PowerShell `Set-Date` on Windows).
- Check the exit status of each step. On Windows, set date and time in one call.
- Map `EPERM` / Windows privilege errors to "need root or SeSystemtimePrivilege" before the raw errno.

`SetSystemTime` on Linux assigns `Usec` from an `int64`. On 32-bit Linux and ARM, `syscall.Timeval.Usec` is `int32`, so that file does not build. Cast through the field's type, or use `golang.org/x/sys/unix` with the right build tags.

**Implemented:** `timeutils/set.go` per-OS commands; `golang.org/x/sys/unix` and `golang.org/x/sys/windows` for syscalls; permission error mapping.

## P1 — results that look fine and are not

### 7. Hostname resolution throws away IPv6, ports, and the pool — **DONE**

`FetchTimeFromNTP` resolves the name with `GetServerIP`, keeps the first IPv4, and queries that address. Effects:

- IPv6-only names fail with `no IPv4 address found`.
- `host:123` fails `LookupIP` because the port is still attached. `ntp.Query` already accepts `host`, `host:port`, and `[ipv6]:port`.
- `pool.ntp.org` is supposed to be queried by name so DNS spreads load. Pinning one address and then opening 10 parallel queries is the pattern pool operators rate-limit.
- The table's Server row is the raw IP. The README still shows `name (ip)`.

**Proposal**

- Pass the user string straight to `ntp.QueryWithOptions`. Resolve only for display.
- If a literal address is required, use `net.ResolveIPAddr` and accept IPv6. Try IPv6 and IPv4, do not stop at the first A record.
- Query pools by hostname. In high-accuracy mode, space samples (for example 200ms apart) instead of a 10-wide burst. Make count, spacing, and timeout flags.

**Implemented:** hostname passed to `QueryWithOptions`; `formatServerDisplay()` for `name (ip)` display only; sequential spaced samples.

### 8. `windows-time` is NTP with a different argument — **DONE**

`windows-time` calls `FetchTimeFromNTP("", server, ...)`, the same path as `ntp`. It does not speak MS-SNTP authentication. Against a domain controller that requires a signed client, it fails or, worse, accepts an unsigned answer and looks successful.

**Proposal**

- Rename the help text to say this is an NTP query aimed at a Windows Time host, or implement real signed MS-SNTP and test it.
- Until then, do not imply a second protocol in the command list.

**Implemented:** help text clarifies unsigned NTP to a Windows Time host.

### 9. High-accuracy mode is not safe to run as the default sync path — **DONE**

`GatherHighAccuracyTime` starts 10 goroutines that each call `ntp.Query` in a retry loop. `ntp.Query` does not take a `context`. The timeout is checked only between attempts, and each attempt may block for the library default of 5 seconds. Failures are printed from the goroutines, so lines interleave with the table.

A short context plus a blocking query means the command can run well past `--timeout`, and a kiss-of-death reply is stored as a sample (see item 3).

**Proposal**

- Use `QueryWithOptions` and a per-attempt timeout shorter than the overall deadline.
- Collect samples on one goroutine, or give each attempt a child context that is actually honored by a custom dialer deadline.
- Stop retrying a server that returns `RATE`.
- Print progress from the collector, not from the workers.
- Extend the table with median offset, median RTT, min/max, and how many samples were kept.

**Implemented:** sequential collector in `gatherHighAccuracyTime()`; per-query timeout; stats in output table.

### 10. Parent flags are silently ignored — **DONE**

`--set`, `--system-tools`, and `--high-accuracy` are registered on the root command and again on each subcommand, on different `commandOptions` values. `ntpcl ntp host --set` works. `ntpcl --set ntp host` sets `rootOpts` and then runs the subcommand, which reads `ntpOpts` and never sets the clock. No error.

**Proposal**

- Make the shared flags `PersistentFlags` on the root command, and keep a single `commandOptions`.
- Ignore `--system-tools` unless `--set` is present, and say so.

**Implemented:** `PersistentFlags` on root; single `opts` struct; warning when `--system-tools` without `--set`.

### 11. README describes a CLI that is gone — **DONE**

`README.md` still documents `--ntp-server`, `--http-server`, a line-oriented report, and a high-accuracy transcript ("Average offset") that the current printer does not produce. The help text is the source of truth; the README will teach people flags that do not exist.

**Proposal**

- Rewrite the examples as `ntpcl`, `ntpcl ntp SERVER --set`, `ntpcl http URL`, `ntpcl daytime SERVER`, `ntpcl time SERVER`.
- Show the table format the program prints now.
- Add install instructions. `go.mod` says `module ntpcl`, so `go install github.com/earentir/ntpcl@latest` cannot work. Change the module path to `github.com/earentir/ntpcl` (or whatever the real repo path is) before tagging a release.

**Implemented:** README rewritten; module path `github.com/earentir/ntpcl`.

## P2 — quality, packaging, and smaller correctness issues

### Output — **DONE**

- `Precision` is a `time.Duration` printed with `%d`, so the cell is a raw nanosecond count. Print it with `String()`, and add leap indicator, version, reference id (`ReferenceString()`), reference time, root distance, and kiss code when present.
- Colored cells go through `tablewriter` as ordinary strings, so ANSI sequences count toward column width and the grid shifts. Color after render, or disable color when the writer is not a terminal. Honor a `--no-color` flag as well as `NO_COLOR`.
- Add `--format json` (one object: method, server, offset, rtt, stratum, valid) and `--format unix` / `--format rfc3339` for scripts. Keep the table as the default.
- `--quiet` should print only the corrected time or nothing on success.

**Implemented:** `timeutils/output.go`; `--format`, `--quiet`, `--no-color`, `NO_COLOR` and TTY detection.

### Dead code and API shape — **DONE**

These are unused from `main`: `QueryNTPTime`, `QuerySNTPTime` (it only calls `QueryNTPTime`), `PrintNTPDetails`, `countNonEmptySources` (each command already sets one source), and the default branch in `fetchTime` (the root command always passes `europe.pool.ntp.org`). `GatherHighAccuracyTime` also prints to stdout, so the library package is not usable as a library.

**Proposal:** fold fetch, sample, and set behind a small `Source` interface that returns `{Time, Offset, RTT, Server, Details, Err}`. Leave formatting in `main`. Delete the unused wrappers or wire SNTP up as a real flag only if it does something `ntp.Query` does not.

**Implemented:** dead code removed; `TimeResult` struct is the unified fetch result; package split into focused files.

### Errors and exit codes — **DONE**

- Error strings are capitalized (`Only one time source...`, `Failed to fetch time`). Go errors should be lowercase so they wrap cleanly.
- `GetServerIP` uses `%v`, which drops `errors.Is` / `errors.As`. Use `%w`.
- Distinguish exit statuses: `2` usage, `3` network, `4` invalid time (kiss-of-death, unreasonable delta), `5` permission. Today everything is `1`.
- `--high-accuracy` is rejected for HTTP with the text "only be used with NTP", but `windows-time` is allowed because it is NTP. Say "NTP sources" or reject the flag on that command too.

**Implemented:** typed errors in `timeutils/errors.go`; exit codes 2–5; message says "NTP sources".

### Clock stepping on each OS — **DONE** (partial documentation)

- Linux `settimeofday` needs `CAP_SYS_TIME`. On systems running `systemd-timesyncd` or `chronyd`, the daemon will step the clock back. Document that, and consider `adjtimex` / `clock_adjtime` for a slew mode (`--slew`) under a small threshold.
- macOS `settimeofday` is restricted. If it fails, the error should name the entitlement or suggest `sudo`, not only the errno.
- Windows should call `SetSystemTime` through `golang.org/x/sys/windows` rather than `syscall.NewLazyDLL`. Millisecond truncation is fine if the UI says so.

**Implemented:** `golang.org/x/sys` syscalls; permission hints in errors. `--slew` / adjtimex not implemented (future work).

### Tests and CI — **DONE**

There are no `*_test.go` files and no GitHub Actions workflow. The parsers and the set-path formatters are pure enough to test without root.

**Proposal, in order:**

1. Table-driven tests for `parseDaytimeResponse`, HTTP date parsing, and the RFC 868 `uint32` conversion, including the 2036 boundary.
2. A UDP test server that only replies after it receives a datagram, so the time-protocol bug cannot regress.
3. NTP tests against a fake UDP listener: normal response, stratum 0, and a response `Validate` rejects. Assert `--set` is not called.
4. Tests for the command builder: GNU `date`, macOS `date`, PowerShell. Do not shell out; compare `exec.Cmd` args.
5. CLI tests through `rootCmd.Execute()` for `ntpcl --set ntp host` versus `ntpcl ntp host --set`.
6. `golangci-lint` in CI. `.trunk/trunk.yaml` still pins Go 1.21 and golangci-lint 1.59.1 while `go.mod` says `go 1.25.1`. Those pins will not lint this module. Bump them together and add a `.golangci.yml`.

**Implemented:** `fetch_test.go`, `ntp_test.go`, `main_test.go`; `.github/workflows/ci.yml`; `.golangci.yml`; trunk pins updated. Command-builder arg tests deferred (syscall path is default).

### Version and release — **SKIPPED (by design)**

`Version: "0.4.19"` is a constant. Stamp it from the build instead:

```text
go build -ldflags "-X main.version=..."
```

and include `runtime/debug` VCS revision in `ntpcl --version`.

**Not implemented:** version remains the explicit `appVersion = "0.4.23"` string per project preference.

## Suggested order of work

| Order | Item | Status |
| --- | --- | --- |
| 1 | Call `Response.Validate()` and refuse kiss-of-death | **DONE** |
| 2 | Max-delta check and `--dry-run` | **DONE** |
| 3 | Deadlines on daytime, time-protocol, and HTTP | **DONE** |
| 4 | Send the RFC 868 request datagram | **DONE** |
| 5 | Re-apply NTP offset at the moment of `settimeofday` | **DONE** |
| 6 | Stop rewriting NTP hosts to a single IPv4 | **DONE** |
| 7 | Serialize high-accuracy samples and surface median RTT | **DONE** |
| 8 | Persistent flags | **DONE** |
| 9 | Rewrite or delete `--system-tools` | **DONE** |
| 10 | README, module path, tests, CI | **DONE** |

## Remaining future work (not in original scope)

- `--slew` gradual adjustment via `adjtimex` / `clock_adjtime`
- MS-SNTP authenticated Windows Time client
- Configurable high-accuracy sample count/spacing via CLI flags
- Command-builder unit tests for `--system-tools` paths
