// Package main batch-register 子命令实现（CSV 输入）。
//
// 用法：
//
//	agentid batch-register --file agents.csv --output credentials.json
//
// CSV 格式（首行为表头；缺省列默认 0/0xFF/"-"）：
//
//	owner,level,permission,public_key
//	did:agentid:alice,1,255,base64-pk
//	did:agentid:bob,2,4095,base64-pk
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agentid-chain/agentid-chain/internal/cli"
	"github.com/agentid-chain/agentid-chain/internal/cli/output"
	"github.com/spf13/cobra"
)

var batchFlagVals = struct {
	file     string
	output   string
	format   string
	config   string
	gateway  string
	apiKey   string
	mode     string
	timeout  int
	parallel int
}{}

var batchCmdImpl = &cobra.Command{
	Use:   "batch-register",
	Short: "批量注册 Agent（CSV 输入）",
	Long: `从 CSV 文件批量注册 Agent；输出凭证 JSON 数组。

CSV 列：owner,level,permission,public_key（首行表头）
例：
  agentid batch-register --file agents.csv --output credentials.json --parallel 4`,
	RunE: runBatchRegister,
}

func init() {
	batchCmdImpl.Flags().StringVar(&batchFlagVals.file, "file", "", "输入 CSV 路径（必填）")
	batchCmdImpl.Flags().StringVar(&batchFlagVals.output, "output", "", "输出凭证 JSON 文件（默认 stdout）")
	batchCmdImpl.Flags().StringVar(&batchFlagVals.format, "format", "", "输出格式")
	batchCmdImpl.Flags().IntVar(&batchFlagVals.parallel, "parallel", 4, "并发数（1-32）")
	batchCmdImpl.Flags().StringVar(&batchFlagVals.config, "config", "", "CLI 配置文件路径")
	batchCmdImpl.Flags().StringVar(&batchFlagVals.gateway, "gateway", "", "gateway 地址")
	batchCmdImpl.Flags().StringVar(&batchFlagVals.apiKey, "api-key", "", "gateway API Key")
	batchCmdImpl.Flags().StringVar(&batchFlagVals.mode, "mode", "", "客户端模式")
	batchCmdImpl.Flags().IntVar(&batchFlagVals.timeout, "timeout", 0, "HTTP 超时（秒）")
}

func runBatchRegister(_ *cobra.Command, _ []string) error {
	if batchFlagVals.file == "" {
		return fmt.Errorf("--file is required")
	}
	if batchFlagVals.parallel < 1 || batchFlagVals.parallel > 32 {
		return fmt.Errorf("--parallel must be in [1, 32]")
	}

	// 1. 解析 CSV
	rows, err := readCSV(batchFlagVals.file)
	if err != nil {
		return fmt.Errorf("read csv: %w", err)
	}
	if len(rows) == 0 {
		return fmt.Errorf("csv is empty")
	}

	// 2. 构造 client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client, err := buildClient(ctx, buildClientOpts{
		ConfigPath: batchFlagVals.config,
		Gateway:    batchFlagVals.gateway,
		APIKey:     batchFlagVals.apiKey,
		Mode:       batchFlagVals.mode,
		Timeout:    batchFlagVals.timeout,
	})
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	// 3. 并发注册
	type result struct {
		row int
		req *cli.RegisterRequest
		cred *cli.AgentCredential
		err  error
	}
	results := make(chan result, len(rows))
	sem := make(chan struct{}, batchFlagVals.parallel)

	for i, row := range rows {
		req, perr := rowToRequest(row)
		if perr != nil {
			results <- result{row: i, err: fmt.Errorf("row %d: %w", i+2, perr)}
			continue
		}
		sem <- struct{}{}
		go func(i int, req *cli.RegisterRequest) {
			defer func() { <-sem }()
			cred, err := client.RegisterAgent(ctx, req)
			results <- result{row: i, req: req, cred: cred, err: err}
		}(i, req)
	}
	// drain
	close(sem)

	out := make([]batchResult, 0, len(rows))
	for j := 0; j < len(rows); j++ {
		r := <-results
		br := batchResult{Row: r.row + 2, Owner: r.req.Owner}
		if r.err != nil {
			br.Error = r.err.Error()
		} else if r.cred != nil {
			br.UUID = r.cred.UUID
			br.Level = r.cred.Level
			br.State = r.cred.State
			br.TxHash = r.cred.TxHash
		}
		out = append(out, br)
	}

	// 4. 输出
	format := output.ParseFormat(chooseFormat(client, batchFlagVals.format))
	return writeBatchOutput(out, batchFlagVals.output, format)
}

// batchResult 单行结果。
type batchResult struct {
	Row    int    `json:"row"`
	Owner  string `json:"owner"`
	UUID   string `json:"uuid,omitempty"`
	Level  uint8  `json:"level,omitempty"`
	State  string `json:"state,omitempty"`
	TxHash string `json:"tx_hash,omitempty"`
	Error  string `json:"error,omitempty"`
}

// rowToRequest 解析单行 CSV。
func rowToRequest(row map[string]string) (*cli.RegisterRequest, error) {
	owner := strings.TrimSpace(row["owner"])
	if owner == "" {
		return nil, fmt.Errorf("empty owner")
	}
	levelStr := strings.TrimSpace(row["level"])
	level := uint8(1)
	if levelStr != "" {
		v, err := strconv.ParseUint(levelStr, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid level %q", levelStr)
		}
		level = uint8(v)
		if level == 0 {
			level = 1
		}
	}
	permStr := strings.TrimSpace(row["permission"])
	perm := uint64(0xFF)
	if permStr != "" {
		v, err := strconv.ParseUint(permStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid permission %q", permStr)
		}
		perm = v
	}
	pk := strings.TrimSpace(row["public_key"])
	if pk == "" {
		return nil, fmt.Errorf("empty public_key")
	}
	return &cli.RegisterRequest{
		Owner:      owner,
		Level:      level,
		Permission: perm,
		PublicKey:  pk,
	}, nil
}

// readCSV 读 CSV → map（首行表头）。
func readCSV(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // 不要求固定列数
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	for i := range header {
		header[i] = strings.ToLower(strings.TrimSpace(header[i]))
	}
	var out []map[string]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		row := map[string]string{}
		for i, h := range header {
			if i < len(rec) {
				row[h] = rec[i]
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// writeBatchOutput 写批结果。
func writeBatchOutput(results []batchResult, path string, format output.Format) error {
	if path == "" || path == "-" {
		return output.Print(prettyWriter(), format, results)
	}
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := output.Print(f, format, results); err != nil {
		return err
	}
	// 统计
	ok, fail := 0, 0
	for _, r := range results {
		if r.Error == "" {
			ok++
		} else {
			fail++
		}
	}
	fmt.Fprintf(os.Stderr, "batch: ok=%d fail=%d → %s\n", ok, fail, path)
	return nil
}

// 抑制 unused warning
var _ = json.Marshal
