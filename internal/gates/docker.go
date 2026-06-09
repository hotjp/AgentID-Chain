package gates

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// NewDockerBuildGate 构造 Docker 构建门控。
//
// 行为：
//   - 在 repoRoot 下构建所有 Dockerfile.dev / Dockerfile.local / Dockerfile
//   - 工具缺失 → 错误
//   - 任一镜像构建失败 → 错误
type DockerBuildGate struct {
	repoRoot  string
	dockerfiles []string
}

// NewDockerBuildGate 默认扫描根目录 + 子目录的 Dockerfile。
func NewDockerBuildGate() *DockerBuildGate {
	return &DockerBuildGate{
		repoRoot: ".",
		dockerfiles: []string{
			"docker-compose.dev.yml", // 提示存在 compose 文件
		},
	}
}

func (g *DockerBuildGate) Name() string        { return "docker_build" }
func (g *DockerBuildGate) Severity() Severity  { return SeverityMandatory }
func (g *DockerBuildGate) Required() bool      { return true }
func (g *DockerBuildGate) Description() string { return "all Dockerfiles must build successfully" }

// Check 扫描并构建所有 Dockerfile。
func (g *DockerBuildGate) Check(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker CLI not found: %w", err)
	}

	// 探测 compose 编排文件存在
	for _, compose := range g.dockerfiles {
		if _, err := fileExists(filepath.Join(g.repoRoot, compose)); err != nil {
			return fmt.Errorf("compose file missing: %s", compose)
		}
	}

	// 扫描所有 Dockerfile*
	dockerfiles, err := glob(filepath.Join(g.repoRoot, "**/Dockerfile*"))
	if err != nil {
		return fmt.Errorf("scan dockerfiles: %w", err)
	}
	if len(dockerfiles) == 0 {
		return fmt.Errorf("no Dockerfile found under %s", g.repoRoot)
	}

	for _, df := range dockerfiles {
		dir := filepath.Dir(df)
		tag := fmt.Sprintf("agentid-chain-gate:%s", filepath.Base(dir))
		cmd := exec.CommandContext(ctx, "docker", "build", "-f", df, "-t", tag, dir)
		var out, errOut bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errOut
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("docker build %s failed: %w\n%s", df, err, truncate(errOut.String(), 3000))
		}
	}
	return nil
}
