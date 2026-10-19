// distribution_handlers.go 实现构建产物分发指令(dist_apply):
// 经预签名 URL 从对象存储下载产物 → (zip 解包/单文件复制)进项目目录
// (白名单校验,防 zip slip)→ 按 afterOpt 联动项目重启/启动。
package agent

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go-wind-ops/pkg/filetransfer"
	"go-wind-ops/pkg/signal"
)

func (s *Service) handleDistApply(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	payload := &signal.DistributionPayload{}
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, payload); err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
	}
	if payload.DownloadUrl == "" || payload.ProjectDir == "" || payload.DistributionId == 0 {
		return nil, fmt.Errorf("payload.download_url/project_dir/distribution_id are required")
	}
	if err := s.checkDir(&signal.ProjectOperationPayload{Dir: payload.ProjectDir}); err != nil {
		return nil, err
	}

	// ── 1. 下载产物到临时文件 ──
	tmpFile, err := os.CreateTemp("", "ops-dist-*"+filepath.Ext(payload.FileName))
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)

	size, sha256hex, err := filetransfer.DownloadToFile(ctx, payload.DownloadUrl, tmpPath)
	if err != nil {
		return nil, fmt.Errorf("download artifact: %w", err)
	}
	s.log().Infof("distribution artifact downloaded (dist=%d file=%s size=%d sha256=%s)",
		payload.DistributionId, payload.FileName, size, sha256hex[:12])

	// ── 2. 清空旧内容(可选) ──
	if payload.ClearOld {
		if err = clearDir(payload.ProjectDir); err != nil {
			return nil, fmt.Errorf("clear project dir: %w", err)
		}
	}
	if err = os.MkdirAll(payload.ProjectDir, 0o755); err != nil {
		return nil, fmt.Errorf("ensure project dir: %w", err)
	}

	// ── 3. 解包/复制进项目目录 ──
	var deployed int
	var output strings.Builder
	switch {
	case strings.EqualFold(filepath.Ext(payload.FileName), ".zip"):
		deployed, err = extractZip(tmpPath, payload.ProjectDir)
		if err != nil {
			return nil, fmt.Errorf("extract zip: %w", err)
		}
		fmt.Fprintf(&output, "extracted %d file(s) from %s", deployed, payload.FileName)
	default:
		target := filepath.Join(payload.ProjectDir, payload.FileName)
		// 目录逃逸复检:FileName 含 ..\ 等相对段时 Join 会折叠出 ProjectDir,
		// 必须确认落点仍在白名单已校验的项目目录内(上游 artifact 键名不滤反斜杠)。
		if !pathWithinDir(payload.ProjectDir, target) {
			return nil, fmt.Errorf("deploy path %q escapes project dir", payload.FileName)
		}
		if err = copyFile(tmpPath, target); err != nil {
			return nil, fmt.Errorf("deploy file: %w", err)
		}
		deployed = 1
		fmt.Fprintf(&output, "deployed %s", payload.FileName)
	}

	// ── 4. 分发后动作 ──
	projectPayload, err := json.Marshal(&signal.ProjectOperationPayload{
		ProjectId:     payload.ProjectId,
		Name:          payload.ProjectName,
		Dir:           payload.ProjectDir,
		StartCommand:  payload.StartCommand,
		StopCommand:   payload.StopCommand,
		StatusCommand: payload.StatusCommand,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal project payload: %w", err)
	}
	projectEnv := &signal.CommandEnvelope{Payload: projectPayload}
	switch payload.AfterOpt {
	case signal.AfterRestart:
		if _, err = s.handleProjectRestart(ctx, projectEnv); err != nil {
			return nil, fmt.Errorf("restart after distribute: %w", err)
		}
		fmt.Fprintf(&output, "\nproject restarted")
	case signal.AfterStart:
		if _, err = s.handleProjectStart(ctx, projectEnv); err != nil {
			return nil, fmt.Errorf("start after distribute: %w", err)
		}
		fmt.Fprintf(&output, "\nproject started")
	}

	return signal.DistributionResult{
		DistributionId: payload.DistributionId,
		DeployedFiles:  deployed,
		Output:         truncateOutput(output.String()),
	}, nil
}

// extractZip 解包 zip 到目标目录;防 zip slip:条目名不得逃逸目标目录。
func extractZip(zipPath, targetDir string) (int, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	targetAbs, err := filepath.Abs(targetDir)
	if err != nil {
		return 0, err
	}

	deployed := 0
	for _, f := range reader.File {
		name := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		if strings.HasPrefix(name, "..") || path.IsAbs(name) {
			return deployed, fmt.Errorf("zip entry escapes target dir: %s", f.Name)
		}
		// Windows 盘符绝对条目("C:/x")在非 Windows 平台的 path.IsAbs 检不出来
		if len(name) >= 2 && name[1] == ':' {
			return deployed, fmt.Errorf("zip entry escapes target dir: %s", f.Name)
		}
		target := filepath.Join(targetAbs, filepath.FromSlash(name))

		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(target, 0o755); err != nil {
				return deployed, err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return deployed, err
		}
		src, err := f.Open()
		if err != nil {
			return deployed, err
		}
		dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			src.Close()
			return deployed, err
		}
		_, err = io.Copy(dst, src)
		src.Close()
		dst.Close()
		if err != nil {
			return deployed, err
		}
		deployed++
	}
	return deployed, nil
}

// clearDir 清空目录内容(保留目录本身)。
func clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if err = os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// pathWithinDir 词法判定 target 是否落在 base 目录内(含 base 本身)。
// 配合 checkDir 使用:checkDir 校验 base 处于授权目录,本函数保证拼装后的
// 具体落点没有经相对段/分隔符技巧逃逸出 base。
func pathWithinDir(base, target string) bool {
	absBase, err1 := filepath.Abs(filepath.Clean(base))
	absTarget, err2 := filepath.Abs(filepath.Clean(target))
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
