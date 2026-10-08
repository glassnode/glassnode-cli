package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glassnode/glassnode-cli/internal/api"
	"github.com/olekukonko/tablewriter"
)

// Options configures output formatting.
type Options struct {
	Format          string      // json, csv, or table
	Data            interface{} // payload to print
	TimestampFormat string      // for table/csv: unix, humanized, or Go layout (e.g. 2006-01-02 15:04:05)
}

// formatTimestamp formats a Unix timestamp (seconds). timestampFormat may be:
// "unix" (raw seconds), "humanized" or "" (2006-01-02 15:04:05), or any Go time layout.
func formatTimestamp(t int64, timestampFormat string) string {
	if timestampFormat == "unix" {
		return fmt.Sprintf("%d", t)
	}
	layout := "2006-01-02 15:04:05"
	if timestampFormat != "" && timestampFormat != "humanized" {
		layout = timestampFormat
	}
	return time.Unix(t, 0).UTC().Format(layout)
}

// Print writes data to os.Stdout according to opts. For tests, use PrintTo with a buffer.
func Print(opts Options) error {
	return PrintTo(os.Stdout, opts)
}

// PrintTo writes opts.Data to w in the format given by opts.
func PrintTo(w io.Writer, opts Options) error {
	switch opts.Format {
	case "json":
		return PrintJSON(w, opts.Data)
	case "csv":
		return PrintCSV(w, opts.Data, opts.TimestampFormat)
	case "table":
		return PrintTable(w, opts.Data, opts.TimestampFormat)
	default:
		return fmt.Errorf("unsupported output format: %s", opts.Format)
	}
}

func PrintJSON(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func PrintCSV(w io.Writer, data interface{}, timestampFormat string) error {
	cw := csv.NewWriter(w)

	write := func(record []string) error {
		return cw.Write(record)
	}

	switch v := data.(type) {
	case []string:
		if err := write([]string{"metric"}); err != nil {
			return err
		}
		for _, s := range v {
			if err := write([]string{s}); err != nil {
				return err
			}
		}
	case []api.Asset:
		if err := write([]string{"id", "symbol", "name", "asset_type", "categories"}); err != nil {
			return err
		}
		for _, a := range v {
			if err := write([]string{a.ID, a.Symbol, a.Name, a.AssetType, strings.Join(a.Categories, ";")}); err != nil {
				return err
			}
		}
	case []map[string]interface{}:
		if len(v) == 0 {
			cw.Flush()
			return cw.Error()
		}
		keys := sortedKeys(v[0])
		if err := write(keys); err != nil {
			return err
		}
		for _, m := range v {
			row := make([]string, len(keys))
			for i, k := range keys {
				row[i] = valueStr(m[k])
			}
			if err := write(row); err != nil {
				return err
			}
		}
	case *api.MetricMetadata:
		return PrintJSON(w, v)
	case []api.DataPoint:
		if len(v) == 0 {
			cw.Flush()
			return cw.Error()
		}
		computed := hasComputedAt(v)
		if v[0].O != nil {
			keys := sortedKeys(v[0].O)
			header := append([]string{"t"}, keys...)
			if computed {
				header = append(header, "computed_at")
			}
			if err := write(header); err != nil {
				return err
			}
			for _, dp := range v {
				row := []string{formatTimestamp(dp.T, timestampFormat)}
				for _, k := range keys {
					row = append(row, valueStr(dp.O[k]))
				}
				if computed {
					row = append(row, computedAtStr(dp, timestampFormat))
				}
				if err := write(row); err != nil {
					return err
				}
			}
		} else {
			header := []string{"t", "v"}
			if computed {
				header = append(header, "computed_at")
			}
			if err := write(header); err != nil {
				return err
			}
			for _, dp := range v {
				row := []string{formatTimestamp(dp.T, timestampFormat), valueStr(dp.V)}
				if computed {
					row = append(row, computedAtStr(dp, timestampFormat))
				}
				if err := write(row); err != nil {
					return err
				}
			}
		}
	case *api.BulkResponse:
		if len(v.Data) == 0 {
			cw.Flush()
			return cw.Error()
		}
		var keys []string
		if len(v.Data) > 0 && len(v.Data[0].Bulk) > 0 {
			keys = sortedKeys(v.Data[0].Bulk[0])
		}
		header := append([]string{"t"}, keys...)
		if err := write(header); err != nil {
			return err
		}
		for _, dp := range v.Data {
			for _, entry := range dp.Bulk {
				row := []string{formatTimestamp(dp.T, timestampFormat)}
				for _, k := range keys {
					row = append(row, valueStr(entry[k]))
				}
				if err := write(row); err != nil {
					return err
				}
			}
		}
	default:
		return PrintJSON(w, data)
	}
	cw.Flush()
	return cw.Error()
}

func PrintTable(w io.Writer, data interface{}, timestampFormat string) error {
	table := tablewriter.NewWriter(w)
	table.SetAutoWrapText(false)
	table.SetBorder(false)

	switch v := data.(type) {
	case []string:
		table.SetHeader([]string{"Metric"})
		for _, s := range v {
			table.Append([]string{s})
		}
	case []api.Asset:
		table.SetHeader([]string{"ID", "Symbol", "Name", "Type", "Categories"})
		for _, a := range v {
			table.Append([]string{a.ID, a.Symbol, a.Name, a.AssetType, strings.Join(a.Categories, ", ")})
		}
	case []map[string]interface{}:
		if len(v) == 0 {
			return nil
		}
		keys := sortedKeys(v[0])
		table.SetHeader(upperAll(keys))
		for _, m := range v {
			row := make([]string, len(keys))
			for i, k := range keys {
				row[i] = valueStr(m[k])
			}
			table.Append(row)
		}
	case *api.MetricMetadata:
		return printMetricMetadataTable(w, v)
	case []api.DataPoint:
		if len(v) == 0 {
			return nil
		}
		computed := hasComputedAt(v)
		if v[0].O != nil {
			keys := sortedKeys(v[0].O)
			header := append([]string{"TIME"}, upperAll(keys)...)
			if computed {
				header = append(header, "COMPUTED AT")
			}
			table.SetHeader(header)
			for _, dp := range v {
				row := []string{formatTimestamp(dp.T, timestampFormat)}
				for _, k := range keys {
					row = append(row, valueStr(dp.O[k]))
				}
				if computed {
					row = append(row, computedAtStr(dp, timestampFormat))
				}
				table.Append(row)
			}
		} else {
			header := []string{"TIME", "VALUE"}
			if computed {
				header = append(header, "COMPUTED AT")
			}
			table.SetHeader(header)
			for _, dp := range v {
				row := []string{formatTimestamp(dp.T, timestampFormat), valueStr(dp.V)}
				if computed {
					row = append(row, computedAtStr(dp, timestampFormat))
				}
				table.Append(row)
			}
		}
	case *api.BulkResponse:
		if len(v.Data) == 0 {
			return nil
		}
		var keys []string
		if len(v.Data[0].Bulk) > 0 {
			keys = sortedKeys(v.Data[0].Bulk[0])
		}
		header := append([]string{"TIME"}, upperAll(keys)...)
		table.SetHeader(header)
		for _, dp := range v.Data {
			for _, entry := range dp.Bulk {
				row := []string{formatTimestamp(dp.T, timestampFormat)}
				for _, k := range keys {
					row = append(row, valueStr(entry[k]))
				}
				table.Append(row)
			}
		}
	default:
		return PrintJSON(w, data)
	}

	table.Render()
	return nil
}

func printMetricMetadataTable(w io.Writer, m *api.MetricMetadata) error {
	fmt.Fprintf(w, "Path:           %s\n", m.Path)
	fmt.Fprintf(w, "Tier:           %d\n", m.Tier)
	fmt.Fprintf(w, "Bulk Supported: %t\n", m.BulkSupported)
	fmt.Fprintf(w, "PIT:            %t\n", m.IsPIT)
	if m.Descriptors != nil {
		if m.Descriptors.Name != "" {
			fmt.Fprintf(w, "Name:           %s\n", m.Descriptors.Name)
		}
		if m.Descriptors.Group != "" {
			fmt.Fprintf(w, "Group:          %s\n", m.Descriptors.Group)
		}
		if len(m.Descriptors.Tags) > 0 {
			fmt.Fprintf(w, "Tags:           %s\n", strings.Join(m.Descriptors.Tags, ", "))
		}
	}
	if m.TimeRange != nil {
		fmt.Fprintf(w, "Timerange:      %d - %d\n", m.TimeRange.Min, m.TimeRange.Max)
	}
	if len(m.Parameters) > 0 {
		fmt.Fprintln(w, "Parameters:")
		for k, vals := range m.Parameters {
			fmt.Fprintf(w, "  %s: %s\n", k, strings.Join(vals, ", "))
		}
	}
	return nil
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// valueStr formats a value for CSV/table. Numbers are written in plain
// decimal notation (1738241421021.0708, not 1.7382414210210708e+12), so they
// load into spreadsheets as numbers; slices of strings are joined with ";".
func valueStr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []string:
		return strings.Join(x, ";")
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case json.Number:
		return x.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// hasComputedAt reports whether any point carries computed_at, in which case
// the column is shown for all of them.
func hasComputedAt(points []api.DataPoint) bool {
	for _, dp := range points {
		if dp.ComputedAt != nil {
			return true
		}
	}
	return false
}

func computedAtStr(dp api.DataPoint, timestampFormat string) string {
	if dp.ComputedAt == nil {
		return ""
	}
	return formatTimestamp(*dp.ComputedAt, timestampFormat)
}

func upperAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToUpper(s)
	}
	return out
}
