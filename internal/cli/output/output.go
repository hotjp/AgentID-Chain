// Package output 提供 CLI 输出格式化（JSON / Table / YAML）。
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Format 输出格式。
type Format string

const (
	FormatJSON  Format = "json"
	FormatTable Format = "table"
	FormatYAML  Format = "yaml"
)

// ParseFormat 解析格式（不区分大小写；空 = json）。
func ParseFormat(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "table":
		return FormatTable
	case "yaml", "yml":
		return FormatYAML
	default:
		return FormatJSON
	}
}

// Print 输出单个对象。
func Print(w io.Writer, format Format, v any) error {
	switch format {
	case FormatTable:
		return printTable(w, v)
	case FormatYAML:
		return printYAML(w, v)
	default:
		return printJSON(w, v)
	}
}

// PrintList 输出列表。
func PrintList[T any](w io.Writer, format Format, items []T, headers []string) error {
	switch format {
	case FormatTable:
		return printListTable(w, items, headers)
	case FormatYAML:
		return printYAML(w, items)
	default:
		return printJSON(w, items)
	}
}

// =============================================================================
// JSON
// =============================================================================

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// =============================================================================
// YAML
// =============================================================================

func printYAML(w io.Writer, v any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return enc.Close()
}

// =============================================================================
// Table
// =============================================================================

func printTable(w io.Writer, v any) error {
	// 简单实现：单个对象转 KV 表
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	m, ok := toStringMap(v)
	if !ok {
		// fallback to JSON
		return printJSON(w, v)
	}
	for _, k := range sortedKeys(m) {
		fmt.Fprintf(tw, "%s:\t%s\n", k, m[k])
	}
	return tw.Flush()
}

func printListTable[T any](w io.Writer, items []T, headers []string) error {
	if len(items) == 0 {
		fmt.Fprintln(w, "(empty)")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if len(headers) > 0 {
		fmt.Fprintln(tw, strings.Join(headers, "\t")+"\t")
	}
	for _, item := range items {
		m, ok := toStringMap(item)
		if !ok {
			continue
		}
		cells := make([]string, len(headers))
		for i, h := range headers {
			cells[i] = m[h]
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t")+"\t")
	}
	return tw.Flush()
}

// toStringMap 把任意 struct 序列化为 key/value 字符串（用于 table）。
func toStringMap(v any) (map[string]string, bool) {
	// 反射字段 → JSON → 重新解析
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, false
	}
	out := make(map[string]string, len(raw))
	for k, val := range raw {
		out[k] = fmt.Sprintf("%v", val)
	}
	return out, true
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 稳定排序
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

// =============================================================================
// 错误
// =============================================================================

// PrintError 输出错误（统一格式：{"error":"..."}）。
func PrintError(w io.Writer, err error) {
	fmt.Fprintf(w, `{"error":%q}`+"\n", err.Error())
	if f, ok := w.(*os.File); ok && f == os.Stderr {
		_ = f.Sync()
	}
}

// MustPrint 确保成功输出（错误时 fallback 到 stderr）。
func MustPrint(w io.Writer, format Format, v any) {
	if err := Print(w, format, v); err != nil {
		fmt.Fprintf(os.Stderr, "output: %v\n", err)
	}
}
