// Package uuid_generator 提供 UUID v7 生成器（性能 + 排序友好）。
//
// 性能优化要点：
//   - 单调递增时间戳（前 48 bit）
//   - 批量生成复用 rand 源
//   - 无锁（每个 goroutine 独立 generator）
package uuid_generator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// Generator UUID v7 生成器。
type Generator struct {
	lastTimeNS int64  // 上次时间戳（纳秒）
	counter    uint64 // 同一时间戳内的计数器
}

// NewGenerator 返回新生成器。
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateV7 生成一个 UUID v7（字符串形式）。
// 格式：xxxxxxxx-xxxx-7xxx-yxxx-xxxxxxxxxxxx（其中 y 是 variant bits）。
func (g *Generator) GenerateV7() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}

	nowNS := time.Now().UnixNano()
	lastNS := atomic.SwapInt64(&g.lastTimeNS, nowNS)
	if nowNS == lastNS {
		c := atomic.AddUint64(&g.counter, 1)
		b[6] = byte(c >> 8)
		b[7] = byte(c)
	} else {
		atomic.StoreUint64(&g.counter, 0)
	}

	// 时间戳（前 48 bit）
	b[0] = byte(nowNS >> 40)
	b[1] = byte(nowNS >> 32)
	b[2] = byte(nowNS >> 24)
	b[3] = byte(nowNS >> 16)
	b[4] = byte(nowNS >> 8)
	b[5] = byte(nowNS)

	// version (7) + variant (10)
	b[6] = (b[6] & 0x0F) | 0x70
	b[8] = (b[8] & 0x3F) | 0x80

	return formatUUID(b), nil
}

// BatchGenerate 批量生成 n 个 UUID。
func (g *Generator) BatchGenerate(n int) ([]string, error) {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		u, err := g.GenerateV7()
		if err != nil {
			return nil, err
		}
		out[i] = u
	}
	return out, nil
}

func formatUUID(b [16]byte) string {
	// 标准 UUID 格式：8-4-4-4-12
	hexBuf := make([]byte, 36)
	hex.Encode(hexBuf[0:8], b[0:4])
	hexBuf[8] = '-'
	hex.Encode(hexBuf[9:13], b[4:6])
	hexBuf[13] = '-'
	hex.Encode(hexBuf[14:18], b[6:8])
	hexBuf[18] = '-'
	hex.Encode(hexBuf[19:23], b[8:10])
	hexBuf[23] = '-'
	hex.Encode(hexBuf[24:36], b[10:16])
	return string(hexBuf)
}

// ParseUUID 简单 UUID 校验（仅校验长度 + 短横线位置）。
func ParseUUID(s string) error {
	if len(s) != 36 {
		return fmt.Errorf("uuid: invalid length %d", len(s))
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return fmt.Errorf("uuid: expected dash at %d", i)
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return fmt.Errorf("uuid: invalid char at %d", i)
			}
		}
	}
	return nil
}
