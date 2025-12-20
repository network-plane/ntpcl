# TODO - ntpcl Improvements

## Network & Reliability

- [ ] **Configurable timeouts** - Add configurable timeout options for all network operations (HTTP, Daytime, Time Protocol, NTP)
- [ ] **Retry logic** - Add configurable retry mechanism for failed requests (currently only high-accuracy mode has retries)
- [ ] **Connection timeouts** - Add explicit connection timeouts for Daytime and Time Protocol operations
- [ ] **Time Protocol implementation fix** - RFC 868 requires sending 4 bytes first; current implementation only reads

## Safety & Validation

- [ ] **Time validation before setting** - Add safety checks:
  - Maximum allowed time difference threshold
  - Reasonableness checks (not too far in past/future)
  - Warning for large time jumps
- [ ] **Dry-run mode** - Add `--dry-run` flag to show what would happen without actually setting time
- [ ] **Gradual time adjustment** - Add `--slew` option to gradually adjust time instead of instant change

## Features & Functionality

- [ ] **Multiple server comparison** - Query multiple servers simultaneously and compare/validate results
- [ ] **Output formats** - Add support for JSON, CSV, or plain text output formats for scripting
- [ ] **Quiet/verbose modes** - Add `--quiet` flag for minimal output and `--verbose` for detailed information
- [ ] **IPv6 support** - Currently only supports IPv4; add IPv6 support
- [ ] **SNTP support** - `QuerySNTPTime` function exists but isn't used; expose it
- [ ] **Configuration file** - Support config file for default servers, timeouts, preferences
- [ ] **Statistics tracking** - Track accuracy over time, drift measurements, etc.

## High Accuracy Mode Improvements

- [ ] **Configurable parameters** - Make sample count and timeout configurable (currently hardcoded: 10 samples, 5 seconds)
- [ ] **Progress indicator** - Show progress during high accuracy sampling
- [ ] **Better statistics** - Add standard deviation, min/max values, confidence intervals

## Output & Display

- [ ] **Machine-readable time output** - Add option to output just the time value (e.g., `--format unix`, `--format iso8601`)
- [ ] **More NTP details** - Display additional NTP information (leap second indicators, reference ID, etc.)
- [ ] **Timezone information** - Show timezone details in output
- [ ] **Comparison view** - Side-by-side comparison of multiple time sources

## Code Quality & Robustness

- [ ] **HTTP date parsing** - Support multiple date header formats (RFC 1123, RFC 850, ANSI C)
- [ ] **Daytime protocol parsing** - Support more date format variations (RFC 867 allows variations)
- [ ] **Error context** - Add more context to error messages (which server, which protocol, etc.)
- [ ] **Permission checks** - Check permissions before attempting to set system time

## CLI Improvements

- [ ] **Server selection** - Add flags for default NTP server pools (e.g., `--pool us`, `--pool asia`)
- [ ] **Time offset display** - Show offset in multiple units (ms, seconds, minutes if large)
- [ ] **Color output control** - Add `--no-color` flag to disable colored output
- [ ] **Version info** - Show build information, Go version, etc. in version output

## Documentation & Usability

- [ ] **Better error messages** - More actionable error messages (e.g., "Permission denied: run with sudo")
- [ ] **Examples in help** - Add usage examples to command help text
- [ ] **Exit codes** - Use proper exit codes for different error conditions

## Performance

- [ ] **Parallel server queries** - When comparing multiple servers, query them in parallel
- [ ] **Connection pooling** - Reuse connections where possible

## Security

- [ ] **Server validation** - Option to validate server certificates for HTTPS connections
- [ ] **Rate limiting** - Prevent accidental DoS of time servers

## Priority Recommendations

High priority items that should be addressed first:

1. **Time validation before setting** (Safety)
2. **Configurable timeouts** (Reliability)
3. **Retry logic** (Reliability)
4. **JSON output format** (Scripting/automation)
5. **Dry-run mode** (Safety/usability)
6. **IPv6 support** (Modern networking)
7. **Fix Time Protocol to send request bytes** (RFC compliance)
