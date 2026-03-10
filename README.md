# Glassnode CLI (`gn`)

[![Test](https://github.com/glassnode/glassnode-cli/actions/workflows/test.yml/badge.svg)](https://github.com/glassnode/glassnode-cli/actions/workflows/test.yml) [![Release](https://github.com/glassnode/glassnode-cli/actions/workflows/release.yml/badge.svg)](https://github.com/glassnode/glassnode-cli/actions/workflows/release.yml)

Command-line interface for the [Glassnode API](https://docs.glassnode.com/)

## Installation

### Install script (one command)

**Linux / macOS**

```bash
curl -sSL https://raw.githubusercontent.com/glassnode/glassnode-cli/main/install.sh | bash
```

Installs the latest release to `/usr/local/bin` (uses `sudo` if needed). To install elsewhere:

```bash
INSTALL_DIR=~/bin curl -sSL https://raw.githubusercontent.com/glassnode/glassnode-cli/main/install.sh | bash
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/glassnode/glassnode-cli/main/install.ps1 | iex
```

Installs to `%LOCALAPPDATA%\glassnode\bin` and adds it to your user `PATH` if needed.

### Manual download

Download the archive for your OS and architecture from [Releases](https://github.com/glassnode/glassnode-cli/releases), extract it, then move the binary into your `PATH` and make it executable (`chmod +x gn` on Unix).

### From source

```bash
go install github.com/glassnode/glassnode-cli@latest
```

The binary from `go install` is named `glassnode-cli`. To get the same `gn` command as the release, build with: `go build -o gn .` and put `gn` in your PATH.

## Quick Start

```bash
# Set your API key
export GLASSNODE_API_KEY=your-key

# List available assets
gn asset list

# Fetch Bitcoin's closing price for the last 30 days
gn metric get market/price_usd_close --asset BTC --since 30d
```

## Authentication

The CLI resolves the API key in the following priority order:

1. `--api-key` flag (highest priority)
2. `GLASSNODE_API_KEY` environment variable
3. `~/.gn/config.yaml` configuration file

To persist the key in the config file:

```bash
gn config set api-key=your-key
```

## Commands

### `gn asset list`

List all available assets.

```bash
gn asset list
gn asset list --filter "asset.semantic_tags.exists(tag,tag=='stablecoin')"
gn asset list --filter "asset.id=='BTC'"
```

Use `--prune` to return only specific fields as an array of objects (e.g. for scripting or piping to `jq`):

```bash
# Only asset IDs (array of objects with one field)
gn asset list --prune id -o json

# ID and symbol
gn asset list -p id,symbol -o json
```

| Flag | Short | Description |
|------|-------|-------------|
| `--filter` | | CEL filter expression |
| `--prune` | `-p` | Comma-separated fields to keep (e.g. `id`, `symbol`, `name`); output is array of objects with only those fields |

### `gn asset describe <id>`

Show details about an asset.

```bash
gn asset describe BTC
```

### `gn metric list`

List all available metrics, optionally filtered by asset.

```bash
gn metric list
gn metric list --asset BTC
```

### `gn metric describe <path>`

Show metadata for a metric: supported assets, intervals, exchanges, currencies, and parameters.

```bash
gn metric describe market/price_usd_close
gn metric describe market/price_usd_close --asset BTC
```

### `gn metric get <path>`

Fetch metric data from the API.

```bash
gn metric get market/price_usd_close --asset BTC --interval 24h
gn metric get market/price_usd_close --asset BTC --since 2024-01-01 --until 2024-02-01
gn metric get indicators/sopr --asset BTC --interval 24h --since 30d
gn metric get distribution/balance_exchanges --asset BTC --exchange binance --currency usd
# Bulk: append /bulk to the path; use -a '*' for all assets or multiple -a for specific ones
gn metric get market/marketcap_usd/bulk -a '*' --since 30d
```

| Flag | Short | Description |
|------|-------|-------------|
| `--asset` | `-a` | Asset symbol (repeatable for bulk metrics; use `*` for all assets) |
| `--since` | `-s` | Start time (ISO date or relative: `30d`, `1h`) |
| `--until` | `-u` | End time (ISO date or relative) |
| `--interval` | `-i` | Resolution (`1h`, `24h`, `1w`, `1month`) |
| `--currency` | `-c` | Currency for the metric (`usd`, `native`) |
| `--exchange` | `-e` | Exchange filter (repeatable for bulk) |
| `--network` | `-n` | Network filter |

For **bulk metrics**, append `/bulk` to the path (e.g. `market/marketcap_usd/bulk`). To pass multiple assets, repeat the `-a` (or `--asset`) flag for each: `-a BTC -a ETH -a SOL`. Use `-a '*'` to request all assets.

### `gn config set key=value`

Set a configuration value.

```bash
gn config set api-key=your-key
gn config set output=csv
```

### `gn config get <key|all>`

Read a configuration value, or all values.

```bash
gn config get api-key
gn config get all
```

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--api-key` | | Glassnode API key |
| `--output` | `-o` | Output format: `json` (default), `csv`, `table` |
| `--dry-run` | | Print the request URL without executing |
| `--timestamp-format` | | Timestamp format in output |

## Output Formats

- **json** (default) — structured JSON, suitable for piping to `jq`
- **csv** — comma-separated values for spreadsheets and data pipelines
- **table** — human-readable ASCII table

```bash
# Pipe JSON to jq
gn metric get market/price_usd_close -a BTC --since 7d | jq '.[].v'

# Export to CSV
gn metric get market/price_usd_close -a BTC --since 30d -o csv > prices.csv

# Quick look in the terminal
gn metric get market/price_usd_close -a BTC --since 7d -o table
```

## Examples

### Discover metrics for an asset

```bash
gn metric list --asset BTC
gn metric describe market/price_usd_close --asset BTC
```

### Preview a request without executing

```bash
gn metric get market/price_usd_close --asset BTC --since 30d --dry-run
```

### Bulk fetch across multiple assets

Append `/bulk` to the metric path. Specify multiple assets by repeating `-a` for each, or use `-a '*'` for all:

```bash
# Multiple specific assets (repeat -a for each)
gn metric get market/marketcap_usd/bulk -a BTC -a ETH -a SOL -s 2024-01-01

# All assets (wildcard)
gn metric get market/marketcap_usd/bulk -a '*' --interval 24h --since 30d
```

### Filter assets by tag

```bash
gn asset list --filter "asset.semantic_tags.exists(tag,tag=='stablecoin')"
```

## Development

### Build from source

```bash
git clone https://github.com/glassnode/glassnode-cli.git
cd glassnode-cli
go build -o gn .
```

### Run tests

```bash
go test ./...
```

## License

[Apache-2.0](LICENSE)
