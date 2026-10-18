// file_handlers.go 实现文件管理指令(file_list/file_read/file_write/file_delete)。
// 所有操作须在 agent 授权目录(AGENT_ALLOWED_DIRS)内。
package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go-wind-ops/pkg/filetransfer"
	"go-wind-ops/pkg/signal"
)

// registerFileHandlers 注册文件管理指令处理器。
func (s *Service) registerFileHandlers() {
	s.RegisterHandler("file_list", s.handleFileList)
	s.RegisterHandler("file_read", s.handleFileRead)
	s.RegisterHandler("file_write", s.handleFileWrite)
	s.RegisterHandler("file_delete", s.handleFileDelete)
	s.RegisterHandler(signal.CommandActionFilePull, s.handleFilePull)
	s.RegisterHandler(signal.CommandActionFilePush, s.handleFilePush)
}

// handleFilePull 大文件通道:把本地文件流式上传到预签名 URL(白名单内)。
func (s *Service) handleFilePull(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	var p signal.FilePullPayload
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &p); err != nil { return nil, fmt.Errorf("decode: %w", err) }
	}
	if p.Path == "" || p.Url == "" { return nil, fmt.Errorf("path/url required") }
	if err := s.checkDir(&signal.ProjectOperationPayload{Dir: filepath.Dir(p.Path)}); err != nil { return nil, err }

	size, sha256hex, err := filetransfer.UploadFileToPresignedURL(ctx, p.Path, p.Url, nil)
	if err != nil { return nil, fmt.Errorf("upload: %w", err) }
	s.log().Infof("file pulled to object storage (path=%s size=%d sha256=%s)", p.Path, size, sha256hex[:12])
	return map[string]any{"path": p.Path, "size": size, "sha256": sha256hex}, nil
}

// handleFilePush 大文件通道:从预签名 URL 下载写入本地路径(白名单内)。
func (s *Service) handleFilePush(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	var p signal.FilePushPayload
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &p); err != nil { return nil, fmt.Errorf("decode: %w", err) }
	}
	if p.Path == "" || p.Url == "" { return nil, fmt.Errorf("path/url required") }
	if err := s.checkDir(&signal.ProjectOperationPayload{Dir: filepath.Dir(p.Path)}); err != nil { return nil, err }
	if err := os.MkdirAll(filepath.Dir(p.Path), 0o755); err != nil { return nil, err }

	size, sha256hex, err := filetransfer.DownloadToFile(ctx, p.Url, p.Path)
	if err != nil { return nil, fmt.Errorf("download: %w", err) }
	s.log().Infof("file pushed from object storage (path=%s size=%d sha256=%s)", p.Path, size, sha256hex[:12])
	return map[string]any{"path": p.Path, "size": size, "sha256": sha256hex}, nil
}

// handleFileList 列出目录内容。
func (s *Service) handleFileList(_ context.Context, env *signal.CommandEnvelope) (any, error) {
	var p struct{ Dir string `json:"dir"` }
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
	}
	if p.Dir == "" { return nil, fmt.Errorf("dir required") }
	if err := s.checkDir(&signal.ProjectOperationPayload{Dir: p.Dir}); err != nil { return nil, err }

	entries, err := os.ReadDir(p.Dir)
	if err != nil { return nil, fmt.Errorf("read dir: %w", err) }

	type entry struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
		IsDir bool `json:"is_dir"`
	}
	var out []entry
	for _, e := range entries {
		info, err := e.Info()
		if err != nil { continue }
		out = append(out, entry{Name: e.Name(), Size: info.Size(), IsDir: e.IsDir()})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir { return out[i].IsDir }
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return map[string]any{"dir": p.Dir, "entries": out}, nil
}

// handleFileRead 读取文件内容(base64,截断到 1MB)。
func (s *Service) handleFileRead(_ context.Context, env *signal.CommandEnvelope) (any, error) {
	var p struct{ Path string `json:"path"` }
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &p); err != nil { return nil, fmt.Errorf("decode: %w", err) }
	}
	if p.Path == "" { return nil, fmt.Errorf("path required") }
	if err := s.checkDir(&signal.ProjectOperationPayload{Dir: filepath.Dir(p.Path)}); err != nil { return nil, err }

	data, err := os.ReadFile(p.Path)
	if err != nil { return nil, fmt.Errorf("read: %w", err) }
	const maxRead = 1 << 20
	truncated := false
	if int64(len(data)) > maxRead { data = data[:maxRead]; truncated = true }
	return map[string]any{"path": p.Path, "size": len(data), "truncated": truncated, "content_base64": base64.StdEncoding.EncodeToString(data)}, nil
}

// handleFileWrite 写入文件(base64 内容,自动建目录)。
func (s *Service) handleFileWrite(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	var p struct {
		Path    string `json:"path"`
		Content string `json:"content"` // base64
	}
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &p); err != nil { return nil, fmt.Errorf("decode: %w", err) }
	}
	if p.Path == "" || p.Content == "" { return nil, fmt.Errorf("path/content required") }
	if err := s.checkDir(&signal.ProjectOperationPayload{Dir: filepath.Dir(p.Path)}); err != nil { return nil, err }

	data, err := base64.StdEncoding.DecodeString(p.Content)
	if err != nil { return nil, fmt.Errorf("decode base64: %w", err) }
	if err = os.MkdirAll(filepath.Dir(p.Path), 0o755); err != nil { return nil, err }
	if err = os.WriteFile(p.Path, data, 0o644); err != nil { return nil, err }
	return map[string]any{"path": p.Path, "size": len(data)}, nil
}

// handleFileDelete 删除文件或目录。
func (s *Service) handleFileDelete(ctx context.Context, env *signal.CommandEnvelope) (any, error) {
	var p struct{ Path string `json:"path"` }
	if len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, &p); err != nil { return nil, fmt.Errorf("decode: %w", err) }
	}
	if p.Path == "" { return nil, fmt.Errorf("path required") }
	if err := s.checkDir(&signal.ProjectOperationPayload{Dir: filepath.Dir(p.Path)}); err != nil { return nil, err }
	if err := os.RemoveAll(p.Path); err != nil { return nil, err }
	return map[string]any{"deleted": p.Path}, nil
}
