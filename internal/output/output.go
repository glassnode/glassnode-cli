package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/glassnode/gn/internal/api"
	"github.com/olekukonko/tablewriter"
)

// Print writes data to os.Stdout in the given format. For tests, use PrintTo with a buffer.
func Print(format string, data interface{}) error {
	return PrintTo(os.Stdout, format, data)
}

// PrintTo writes data to w in the given format. Use this in tests with a *bytes.Buffer.
func PrintTo(w io.Writer, format string, data interface{}) error {
	switch format {
	case "json":
		return PrintJSON(w, data)
	case "csv":
		return PrintCSV(w, data)
	case "table":
		return PrintTable(w, data)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func PrintJSON(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func PrintCSV(w io.Writer, data interface{}) error {
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
		if v[0].O != nil {
			keys := sortedKeys(v[0].O)
			header := append([]string{"t"}, keys...)
			if err := write(header); err != nil {
				return err
			}
			for _, dp := range v {
				row := []string{fmt.Sprintf("%d", dp.T)}
				for _, k := range keys {
					row = append(row, fmt.Sprintf("%v", dp.O[k]))
				}
				if err := write(row); err != nil {
					return err
				}
			}
		} else {
			if err := write([]string{"t", "v"}); err != nil {
				return err
			}
			for _, dp := range v {
				if err := write([]string{fmt.Sprintf("%d", dp.T), fmt.Sprintf("%v", dp.V)}); err != nil {
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
				row := []string{fmt.Sprintf("%d", dp.T)}
				for _, k := range keys {
					row = append(row, fmt.Sprintf("%v", entry[k]))
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

func PrintTable(w io.Writer, data interface{}) error {
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
		if v[0].O != nil {
			keys := sortedKeys(v[0].O)
			header := append([]string{"T"}, upperAll(keys)...)
			table.SetHeader(header)
			for _, dp := range v {
				row := []string{fmt.Sprintf("%d", dp.T)}
				for _, k := range keys {
					row = append(row, fmt.Sprintf("%v", dp.O[k]))
				}
				table.Append(row)
			}
		} else {
			table.SetHeader([]string{"T", "V"})
			for _, dp := range v {
				table.Append([]string{fmt.Sprintf("%d", dp.T), fmt.Sprintf("%v", dp.V)})
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
		header := append([]string{"T"}, upperAll(keys)...)
		table.SetHeader(header)
		for _, dp := range v.Data {
			for _, entry := range dp.Bulk {
				row := []string{fmt.Sprintf("%d", dp.T)}
				for _, k := range keys {
					row = append(row, fmt.Sprintf("%v", entry[k]))
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
	fmt.Fprintf(w, "Tier:           %.0f\n", m.Tier)
	fmt.Fprintf(w, "Bulk Supported: %t\n", m.BulkSupported)
	fmt.Fprintf(w, "PIT:            %t\n", m.IsPit)
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
	if m.Timerange != nil {
		fmt.Fprintf(w, "Timerange:      %d - %d\n", m.Timerange.Min, m.Timerange.Max)
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

// valueStr formats a value for CSV/table. Slices of strings are joined with ";".
func valueStr(v interface{}) string {
	if v == nil {
		return ""
	}
	if ss, ok := v.([]string); ok {
		return strings.Join(ss, ";")
	}
	return fmt.Sprintf("%v", v)
}

func upperAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToUpper(s)
	}
	return out
}
