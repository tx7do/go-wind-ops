// backup_handlers.go 实现 backup_run 指令:把授权目录内的源目录打包 zip,
// 经 MinIO 预签名 URL 流式直传对象存储,回执带大小/SHA-256/文件数。
package agent

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"go-wind-ops/pkg/filetransfer"
	"go-wind-ops/pkg/signal"
)

// registerBackupHandlers 注册备份指令处理器。
func (s *Service) registerBackupHandlers() {
	s.RegisterHandler(signal.CommandActionBackupRun, s.handleBackupRun)
}

// handleBackupRun 打包 SourceDir 为 zip(内部路径相对源目录)并直传预签名 URL。
// 源目录须在授权目录白名单内;临时包文件即建即删。
func (s *Service) handleBackupRun(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	var p signal.BackupRunPayload
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
	}
	if p.SourceDir == "" || p.UploadURL == "" {
		return nil, fmt.Errorf("sourceDir/uploadUrl required")
	}
	if err := s.checkDir(&signal.ProjectOperationPayload{Dir: p.SourceDir}); err != nil {
		return nil, err
	}

	start := time.Now()
	tmpFile, err := os.CreateTemp("", "ops-backup-*.zip")
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	fileCount, err := zipDir(tmpFile, p.SourceDir)
	closeErr := tmpFile.Close()
	if err != nil {
		return nil, fmt.Errorf("zip dir: %w", err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close temp: %w", closeErr)
	}

	size, sha256hex, err := filetransfer.UploadFileToPresignedURL(ctx, tmpPath, p.UploadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}

	s.log().Infof("backup uploaded (task=%d size=%d files=%d sha256=%s duration=%s)",
		p.TaskID, size, fileCount, sha256hex[:12], time.Since(start).Truncate(time.Millisecond))
	return &signal.BackupRunResult{
		TaskID:    p.TaskID,
		RecordID:  p.RecordID,
		SizeBytes: size,
		SHA256:    sha256hex,
		FileCount: fileCount,
		Duration:  time.Since(start).Truncate(time.Millisecond).String(),
	}, nil
}

// zipDir 递归打包目录(仅普通文件;符号链接跳过),返回打包文件数。
func zipDir(dst *os.File, srcDir string) (int, error) {
	srcDir = filepath.Clean(srcDir)
	writer := zip.NewWriter(dst)
	count := 0

	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // 符号链接等非常规文件跳过
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		arcName, err := filepath.Rel(srcDir, path)
		if err != nil {
			arcName = d.Name()
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(arcName)
		header.Method = zip.Deflate
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(entry, file)
		_ = file.Close()
		if err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		_ = writer.Close()
		return count, err
	}
	return count, writer.Close()
}
