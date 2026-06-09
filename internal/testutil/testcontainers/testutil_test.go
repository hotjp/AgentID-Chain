package testcontainers

import "os"

// writeFileImpl 独立出来便于真实文件系统调用（避免被 mock）。
func writeFileImpl(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
