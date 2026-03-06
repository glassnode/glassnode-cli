---
name: glassnode-cli
version: 1.0.0
description: Fetch on-chain crypto metrics and market data via the Glassnode CLI
tags:
  - crypto
  - on-chain
  - market-data
  - blockchain
  - api
category: data
metadata:
  openclaw:
    requires:
      bins: ["gn"]
      env: ["GLASSNODE_API_KEY"]
---

# Glassnode CLI (`gn`)

Command-line interface for the Glassnode API — on-chain and crypto market data.

## Setup

**Linux / macOS** (one-liner):
```bash
curl -sSL https://raw.githubusercontent.com/glassnode/gn/main/install.sh | bash
```

**Windows (PowerShell)**:
```powershell
irm https://raw.githubusercontent.com/glassnode/gn/main/install.ps1 | iex
```

Or download manually from [GitHub Releases](https://github.com/glassnode/gn/releases).

Set your API key:
```bash
export GLASSNODE_API_KEY=your-key
# or persist it:
gn config set api-key=your-key
```

## Commands

### List assets
```bash
gn asset list
gn asset list --filter "asset.semantic_tags.exists(tag,tag=='stablecoin')"
gn asset list --filter "asset.id=='BTC'"
# Prune to specific fields (returns array of objects)
gn asset list --prune id -o json
gn asset list -p id,symbol -o json
```

### Describe an asset
```bash
gn asset describe BTC
```

### List metrics
```bash
gn metric list
gn metric list --asset BTC
```

### Describe a metric (discover valid parameters)
```bash
gn metric describe market/price_usd_close
gn metric describe market/price_usd_close --asset BTC
```

### Fetch metric data
```bash
gn metric get market/price_usd_close --asset BTC --interval 24h
gn metric get market/price_usd_close --asset BTC --since 2024-01-01 --until 2024-02-01
gn metric get market/price_usd_close --asset BTC --since 30d
gn metric get indicators/sopr --asset BTC --interval 24h --since 30d
gn metric get distribution/balance_exchanges --asset BTC --exchange binance --currency usd
```

### Bulk fetch (multiple assets)
Append `/bulk` to the metric path. Repeat `-a` (or `--asset`) for each asset, or use `-a '*'` for all:
```bash
gn metric get market/marketcap_usd/bulk -a BTC -a ETH -a SOL -s 1d
gn metric get market/marketcap_usd/bulk -a '*' -s 1d
```

### Config set / get
```bash
gn config set api-key=your-key
gn config set output=csv
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

## Best Practices

- Always call `gn metric describe <path>` before `gn metric get` to discover valid parameters, assets, exchanges, and intervals for a metric.
- Use `--output json` (default) for structured output suitable for piping to `jq`.
- Use `--dry-run` to preview the API request URL without executing.
- For bulk metrics (path ending in `/bulk`), use `-a` with specific assets or `-a '*'` for all.
- Relative time values are supported for `--since` and `--until`: e.g. `30d`, `7d`, `1h`.

## Output Formats

- `--output json` (default) — JSON, suitable for `jq` processing
- `--output csv` — CSV format, suitable for spreadsheets
- `--output table` — human-readable ASCII table

```bash
# Pipe JSON to jq
gn metric get market/price_usd_close -a BTC --since 7d | jq '.[].v'

# Export to CSV
gn metric get market/price_usd_close -a BTC --since 30d -o csv > prices.csv

# Quick look in the terminal
gn metric get market/price_usd_close -a BTC --since 7d -o table
```
