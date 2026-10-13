---
name: glassnode-cli
description: "Use the Glassnode CLI (gn) to list assets and metrics, fetch on-chain and market data from the Glassnode API, check API credit usage, and manage config. Use when the user asks about Glassnode, on-chain data, crypto metrics, gn commands, or needs to call the Glassnode API from the terminal."
---

# Glassnode CLI (`gn`)

Command-line interface for the Glassnode API.

## Setup

If the `gn` command is not available you need to install the CLI. See the [Installation section in the README](https://github.com/glassnode/glassnode-cli/blob/main/README.md#installation) for install and setup options.

Quick start:
```bash
gn asset list
gn metric get market/price_usd_close --asset BTC --since 30d
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
gn metric list -a BTC -e binance -i 24h
gn metric list --assets BTC --assets ETH
gn metric list --from-exchange binance --to-exchange coinbase
```
Filter by any [metadata query parameters](https://docs.glassnode.com/basic-api/metadata#query-parameters-1): `--currency`, `--exchange`, `--interval`, `--from-exchange`, `--to-exchange`, `--miner`, `--maturity`, `--network`, `--period`, `--quote-symbol`, or multiple `--assets`.

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
Append `/bulk` to the metric path. Repeat `-a` (or `--asset`) for each asset, or use `-a '*'` for all.
`--since` is required: the API caps one bulk request at 10 days for `10m`/`1h`, 31 days for `24h`
(the default) and 93 days for `1w`/`1month`, counted from the start of the interval containing
`--since` (weeks start on Monday, months on the 1st, UTC). A longer range is rejected before any request is sent
unless `--split` is passed; the CLI then fetches consecutive windows, prints the number of requests
on stderr, and aborts on any error naming the window and the `--since` to retry from. Each window
is a billed request. `--split` on a non-bulk path is ignored with a note on stderr.
```bash
gn metric get market/marketcap_usd/bulk -a BTC -a ETH -a SOL --since 7d
gn metric get market/marketcap_usd/bulk -a '*' --interval 24h --since 30d
gn metric get market/marketcap_usd/bulk -a BTC -a ETH --since 2024-01-01 --until 2024-04-01 --split
```

### API credit usage (`gn user credits`)

Shows the current API usage for the account.

```bash
gn user credits
```

Example JSON output:

```json
{
  "creditsLeft": integer,
  "creditsLimit": integer,
  "creditsPeriod": "daily" | "monthly",
  "creditsUsed": integer
}
```

All four fields describe the same window. `creditsLimit` is the allowance per
`creditsPeriod` (`daily` on the advanced product, `monthly` on every other one),
and `creditsUsed` and `creditsLeft` are the usage and remainder within it; on a
daily allowance the unit is requests. Never report the limit or the usage as
monthly without reading `creditsPeriod`. If `creditsPeriod` is anything else
(or `mixed`), `creditsUsed` and `creditsLeft` are absent: say the usage is
unknown instead of computing it.

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
| `--dry-run` | | Print the endpoint and parameters without sending the request; the API key goes in the `X-Api-Key` header, not the URL. A preview built from the flags: the SDK appends `f=json` to metric requests and validates parameters only when sending |
| `--timestamp-format` | | Timestamp format in output |

## Best Practices

- Always call `gn metric describe <path>` before `gn metric get` to discover valid parameters, assets, exchanges, and intervals for a metric.
- Use `--output json` (default) for structured output suitable for piping to `jq`.
- Use `--dry-run` to preview the request without sending it: `gn metric get market/price_usd_close --asset BTC --since 30d --dry-run`. The URL has no API key; it is sent in the `X-Api-Key` header.
- For bulk metrics (path ending in `/bulk`), use `-a` with specific assets or `-a '*'` for all. `--since` is required; pass `--split` to fetch a range longer than the per-request cap in several billed requests.
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
