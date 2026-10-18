// Package filetransfer 实现 agent 侧与对象存储(MinIO/S3)之间经预签名 URL
// 的流式直传直拉。文件内容不经过 MQTT,也不经过 core 服务——core 只在信令中
// 下发预签名 URL,agent 直接与对象存储 HTTP 交互,完成后回传对账信息。
package filetransfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultTimeout = 30 * time.Minute
)

// UploadFileToPresignedURL 以流式 PUT 把本地文件直传到预签名 URL,
// 返回传输字节数与 SHA-256(hex)。文件不整读进内存。
func UploadFileToPresignedURL(ctx context.Context, filePath, url string, headers map[string]string) (int64, string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	reader := io.TeeReader(f, hasher)

	uploadCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(uploadCtx, http.MethodPut, url, reader)
	if err != nil {
		return 0, "", fmt.Errorf("new request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("upload: %w", err)
	}
	defer drainClose(resp.Body)

	if resp.StatusCode/100 != 2 {
		return 0, "", fmt.Errorf("upload: unexpected status %s", resp.Status)
	}

	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, "", fmt.Errorf("file size: %w", err)
	}
	return size, hex.EncodeToString(hasher.Sum(nil)), nil
}

// DownloadToFile 流式拉取预签名 URL 到本地文件,返回传输字节数与 SHA-256(hex)。
func DownloadToFile(ctx context.Context, url, filePath string) (int64, string, error) {
	downloadCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, "", fmt.Errorf("new request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("download: %w", err)
	}
	defer drainClose(resp.Body)

	if resp.StatusCode/100 != 2 {
		return 0, "", fmt.Errorf("download: unexpected status %s", resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return 0, "", fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(filePath)
	if err != nil {
		return 0, "", fmt.Errorf("create file: %w", err)
	}

	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, hasher), resp.Body)
	closeErr := f.Close()
	if err != nil {
		return 0, "", fmt.Errorf("write file: %w", err)
	}
	if closeErr != nil {
		return 0, "", fmt.Errorf("close file: %w", closeErr)
	}
	return size, hex.EncodeToString(hasher.Sum(nil)), nil
}

// drainClose 读完剩余响应体后关闭,保证连接可复用。
func drainClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 64<<10))
	_ = body.Close()
}
