package gates

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// fileExists 报告 path 是否存在且可读。
func fileExists(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// readFile 读取文件，限制大小（>10MB 拒绝）。
func readFile(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if fi.Size() > 10*1024*1024 {
		return "", errFileTooLarge{path: path, size: fi.Size()}
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var sb strings.Builder
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteByte('\n')
	}
	return sb.String(), scanner.Err()
}

type errFileTooLarge struct {
	path string
	size int64
}

func (e errFileTooLarge) Error() string {
	return "file too large"
}

// glob 递归 glob 模式（** 支持）。
// 简化实现：filepath.WalkDir + strings.HasPrefix 匹配。
func glob(pattern string) ([]string, error) {
	// 把 ** 拆分为目录前缀
	idx := strings.Index(pattern, "/**/")
	root := pattern
	suffix := ""
	if idx >= 0 {
		root = pattern[:idx]
		suffix = pattern[idx+4:]
	} else if strings.HasSuffix(pattern, "/**") {
		root = pattern[:len(pattern)-3]
	} else if strings.HasSuffix(pattern, "/*") {
		root = pattern[:len(pattern)-2]
	}
	_ = suffix

	var matches []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		// 简化：检查 basename 与 pattern 通配
		base := filepath.Base(p)
		if matchBase(base, filepath.Base(pattern)) {
			matches = append(matches, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// matchBase 简单通配符匹配（只支持 * 和 ?）。
func matchBase(name, pattern string) bool {
	if pattern == "" {
		return true
	}
	// 处理前导 *
	if strings.HasPrefix(pattern, "Dockerfile") {
		return strings.HasPrefix(name, "Dockerfile")
	}
	// 完全匹配 / 子串匹配
	return name == pattern
}
