# ntpcl

A simple time client to fetch and optionally set system time from NTP, HTTP, Daytime, or Time Protocol sources.

## Install

```bash
go install github.com/earentir/ntpcl@latest
```

Or build from source:

```bash
git clone https://github.com/earentir/ntpcl.git
cd ntpcl
go build -o ntpcl .
```

## Usage

### Default (NTP pool)

```bash
./ntpcl
```

Queries `europe.pool.ntp.org` and prints a table with server time, local time, offset, RTT, and NTP details.

### Set system time from NTP

```bash
sudo ./ntpcl --set
./ntpcl ntp pool.ntp.org --set
./ntpcl --set ntp pool.ntp.org
```

Persistent flags work before or after the subcommand.

### High accuracy mode

```bash
./ntpcl ntp pool.ntp.org --high-accuracy
sudo ./ntpcl ntp pool.ntp.org --high-accuracy --set
```

Collects multiple spaced NTP samples, drops outliers, and uses the median offset.

### Other sources

```bash
./ntpcl http https://example.com
./ntpcl daytime time.nist.gov
./ntpcl time time.nist.gov
./ntpcl windows-time dc.example.com
```

`windows-time` queries a Windows Time host over unsigned NTP. It is not MS-SNTP authenticated.

### Safety flags

```bash
./ntpcl --dry-run --set ntp pool.ntp.org
./ntpcl --max-delta 5s --set ntp pool.ntp.org
./ntpcl --force --set http https://example.com
```

- `--dry-run` shows what would be set without changing the clock
- `--max-delta` refuses large clock steps (default 1s; 100ms in high-accuracy mode)
- `--force` overrides max-delta and high-RTT checks for coarse sources

### Output formats

```bash
./ntpcl --format json ntp pool.ntp.org
./ntpcl --format unix ntp pool.ntp.org
./ntpcl --format rfc3339 --quiet ntp pool.ntp.org
./ntpcl --no-color ntp pool.ntp.org
```

### Set via system commands

```bash
sudo ./ntpcl --set --system-tools ntp pool.ntp.org
```

Uses `date` on Linux/macOS or PowerShell `Set-Date` on Windows instead of direct syscalls.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | General error |
| 2 | Usage error |
| 3 | Network error |
| 4 | Invalid time / refused clock step |
| 5 | Permission denied |

## License

MIT — see [LICENSE](LICENSE).
