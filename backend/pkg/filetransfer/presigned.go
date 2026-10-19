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
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout = 30 * time.Minute
)

// UploadFileToPresignedURL 以流式 PUT 把本地文件直传到预签名 URL,
// 返回传输字节数与 SHA-256(hex)。文件不整读进内存。
func UploadFileToPresignedURL(ctx context.Context, filePath, url string, headers map[string]string) (int64, string, error) {
	if err := checkEgressHost(url); err != nil {
		return 0, "", err
	}
	f, err := os.Open(filePath)
	if err != nil {
		return 0, "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, "", fmt.Errorf("stat file: %w", err)
	}

	hasher := sha256.New()
	reader := io.TeeReader(f, hasher)

	uploadCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(uploadCtx, http.MethodPut, url, reader)
	if err != nil {
		return 0, "", fmt.Errorf("new request: %w", err)
	}
	// body 经 TeeReader 包装后 http 层探测不到长度,会走 chunked 编码,
	// MinIO/S3 对 PUT 要求 Content-Length(否则 411),必须显式声明。
	req.ContentLength = info.Size()
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
	if err := checkEgressHost(url); err != nil {
		return 0, "", err
	}

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

// ── 出站主机白名单(纵深防御) ──
//
// 预签名直传直拉是 agent 唯一需要的出站 HTTP;若指令载荷里的 URL 被构造成
// 指向内网/外部地址,则构成 SSRF(file_pull 可外带白名单内文件)。部署侧经
// AGENT_DOWNLOAD_ALLOWED_HOSTS(逗号分隔 host)限定目标;未配置时不限制
// (保持向后兼容),agent 启动时会打一次告警。

var (
	egressMu     sync.RWMutex
	egressHosts  []string
)

// ConfigureAllowedHosts 限定预签名下载/上传的目标 host(空切分项忽略)。
func ConfigureAllowedHosts(hosts []string) {
	cleaned := make([]string, 0, len(hosts))
	for _, h := range hosts {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			cleaned = append(cleaned, h)
		}
	}
	egressMu.Lock()
	egressHosts = cleaned
	egressMu.Unlock()
}

func egressHostsSnapshot() []string {
	egressMu.RLock()
	defer egressMu.RUnlock()
	return append([]string(nil), egressHosts...)
}

// checkEgressHost 校验 URL host 在白名单内;白名单未配置时放行。
func checkEgressHost(rawURL string) error {
	hosts := egressHostsSnapshot()
	if len(hosts) == 0 {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("egress url parse: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range hosts {
		if host == h {
			return nil
		}
	}
	return fmt.Errorf("egress host %q not in allowed list", host)
}
