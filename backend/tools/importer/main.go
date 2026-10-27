// importer 迁移导入工具:读 JSON 清单,经 admin REST 登录后批量创建脚本与
// 项目(按名幂等——已存在跳过)。用于从既有运维平台/手工环境向本平台迁移
// 基础资产。节点不导入:agent 经 TOFU 自注册,项目引用的 nodeId 以清单里
// 显式数字为准(先起 agent 再导项目)。
//
// 清单格式(manifest.json):
//
//	{
//	  "scripts": [
//	    {"name": "deploy.sh", "description": "部署", "content": "#!/bin/bash\n...",
//	     "timeoutSeconds": 60}
//	  ],
//	  "projects": [
//	    {"name": "web-prod", "nodeId": 1, "projectDir": "/opt/web", "env": "prod",
//	     "startCommand": "./start.sh", "stopCommand": "./stop.sh",
//	     "statusCommand": "", "description": "官网", "labels": ["web"]}
//	  ]
//	}
//
// 用法:
//
//	IMPORT_PASSWORD=xxx LOGIN_AES_KEY=<16字节与前端一致> \
//	  go run ./tools/importer -server http://127.0.0.1:18600 \
//	    -username admin -manifest manifest.json
//
// 登录口令经 AES-128-CBC 密文传输(key=iv=LOGIN_AES_KEY,与前端 VITE_AES_KEY
// 同规);验证码需环境关闭(ADMIN_CAPTCHA_DISABLED=true)。
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Manifest 导入清单。
type Manifest struct {
	Scripts  []ScriptSeed  `json:"scripts"`
	Projects []ProjectSeed `json:"projects"`
}

// ScriptSeed 脚本种子。
type ScriptSeed struct {
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	Content        string `json:"content"`
	TimeoutSeconds uint32 `json:"timeoutSeconds,omitempty"`
}

// ProjectSeed 项目种子。
type ProjectSeed struct {
	Name          string   `json:"name"`
	NodeID        uint32   `json:"nodeId"`
	ProjectDir    string   `json:"projectDir"`
	Env           string   `json:"env,omitempty"`
	StartCommand  string   `json:"startCommand,omitempty"`
	StopCommand   string   `json:"stopCommand,omitempty"`
	StatusCommand string   `json:"statusCommand,omitempty"`
	Description   string   `json:"description,omitempty"`
	Labels        []string `json:"labels,omitempty"`
}

// parseManifest 解析清单(独立成函数便于单测)。
func parseManifest(raw []byte) (Manifest, error) {
	var mf Manifest
	err := json.Unmarshal(raw, &mf)
	return mf, err
}

func main() {
	server := flag.String("server", "http://127.0.0.1:18600", "admin REST 地址")
	username := flag.String("username", "admin", "登录用户名")
	manifestPath := flag.String("manifest", "manifest.json", "导入清单 JSON")
	password := flag.String("password", os.Getenv("IMPORT_PASSWORD"), "登录口令(缺省读 IMPORT_PASSWORD)")
	aesKey := flag.String("aes-key", os.Getenv("LOGIN_AES_KEY"), "口令传输 AES 密钥(16 字节,与前端 VITE_AES_KEY 一致;缺省读 LOGIN_AES_KEY)")
	flag.Parse()

	if *password == "" || *aesKey == "" {
		fmt.Fprintln(os.Stderr, "缺少 IMPORT_PASSWORD 或 LOGIN_AES_KEY 环境变量")
		os.Exit(1)
	}
	if len(*aesKey) != 16 {
		fmt.Fprintf(os.Stderr, "LOGIN_AES_KEY 须为 16 字节(AES-128),当前 %d\n", len(*aesKey))
		os.Exit(1)
	}

	raw, err := os.ReadFile(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read manifest: %v\n", err)
		os.Exit(1)
	}
	var mf Manifest
	if err = json.Unmarshal(raw, &mf); err != nil {
		fmt.Fprintf(os.Stderr, "parse manifest: %v\n", err)
		os.Exit(1)
	}

	c := &client{
		base: strings.TrimRight(*server, "/"),
		http: &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: os.Getenv("IMPORT_INSECURE_TLS") == "1"}, //nolint:gosec // 自签 dev 场景显式开关
		}},
	}

	token, err := c.login(*username, *password, *aesKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "login: %v\n", err)
		os.Exit(1)
	}
	c.token = token

	exit := 0
	if err = c.importScripts(mf.Scripts); err != nil {
		fmt.Fprintf(os.Stderr, "import scripts: %v\n", err)
		exit = 1
	}
	if err = c.importProjects(mf.Projects); err != nil {
		fmt.Fprintf(os.Stderr, "import projects: %v\n", err)
		exit = 1
	}
	os.Exit(exit)
}

// ── HTTP 客户端 ──

type client struct {
	base  string
	token string
	http  *http.Client
}

func (c *client) do(method, path string, body any) ([]byte, int, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rd)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	return b, resp.StatusCode, err
}

// login 登录取 token(口令 AES-128-CBC 密文,key=iv=aesKey,PKCS7,base64)。
func (c *client) login(username, password, aesKey string) (string, error) {
	enc, err := encryptPassword(password, aesKey)
	if err != nil {
		return "", err
	}
	body, status, err := c.do(http.MethodPost, "/admin/v1/login", map[string]string{
		"grant_type":  "password",
		"username":    username,
		"password":    enc,
		"client_type": "admin",
	})
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", status, truncate(body))
	}
	var resp struct {
		AccessToken string `json:"access_token"`
	}
	if err = json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("empty access_token: %s", truncate(body))
	}
	return resp.AccessToken, nil
}

// listNames 拉全量名称(分页 pageSize=500,超出 500 条的部署请分批导入)。
func (c *client) listNames(path string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	for page := uint32(1); ; page++ {
		body, status, err := c.do(http.MethodGet,
			fmt.Sprintf("%s?page=%d&pageSize=500", path, page), nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("%s status %d: %s", path, status, truncate(body))
		}
		var resp struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
			Total int `json:"total"`
		}
		if err = json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		for _, it := range resp.Items {
			out[it.Name] = struct{}{}
		}
		if uint32(len(resp.Items)) == 0 || uint32(page*500) >= uint32(resp.Total) {
			return out, nil
		}
	}
}

type importResult struct {
	created int
	skipped int
	failed  int
}

func (r importResult) String() string {
	return fmt.Sprintf("created=%d skipped=%d failed=%d", r.created, r.skipped, r.failed)
}

func (c *client) importScripts(seeds []ScriptSeed) error {
	existing, err := c.listNames("/admin/v1/scripts")
	if err != nil {
		return err
	}
	var res importResult
	for _, s := range seeds {
		if _, dup := existing[s.Name]; dup {
			res.skipped++
			fmt.Printf("script [%s] 已存在,跳过\n", s.Name)
			continue
		}
		body, status, err := c.do(http.MethodPost, "/admin/v1/scripts", map[string]any{
			"data": map[string]any{
				"name":           s.Name,
				"description":    s.Description,
				"content":        s.Content,
				"timeoutSeconds": s.TimeoutSeconds,
			},
		})
		if err != nil || status != http.StatusOK {
			res.failed++
				fmt.Printf("script [%s] 创建失败: status=%d err=%v body=%s\n", s.Name, status, err, truncate(body))
			continue
		}
		res.created++
		fmt.Printf("script [%s] 已创建\n", s.Name)
	}
	fmt.Println("scripts:", res)
	if res.failed > 0 {
		return fmt.Errorf("%d script(s) failed", res.failed)
	}
	return nil
}

func (c *client) importProjects(seeds []ProjectSeed) error {
	existing, err := c.listNames("/admin/v1/projects")
	if err != nil {
		return err
	}
	var res importResult
	for _, p := range seeds {
		if _, dup := existing[p.Name]; dup {
			res.skipped++
			fmt.Printf("project [%s] 已存在,跳过\n", p.Name)
			continue
		}
		if p.NodeID == 0 || p.ProjectDir == "" {
			res.failed++
				fmt.Printf("project [%s] 缺 nodeId/projectDir,跳过创建\n", p.Name)
			continue
		}
		body, status, err := c.do(http.MethodPost, "/admin/v1/projects", map[string]any{
			"data": map[string]any{
				"name":          p.Name,
				"nodeId":        p.NodeID,
				"projectDir":    p.ProjectDir,
				"env":           p.Env,
				"startCommand":  p.StartCommand,
				"stopCommand":   p.StopCommand,
				"statusCommand": p.StatusCommand,
				"description":   p.Description,
				"labels":        p.Labels,
			},
		})
		if err != nil || status != http.StatusOK {
			res.failed++
				fmt.Printf("project [%s] 创建失败: status=%d err=%v body=%s\n", p.Name, status, err, truncate(body))
			continue
		}
		res.created++
		fmt.Printf("project [%s] 已创建\n", p.Name)
	}
	fmt.Println("projects:", res)
	if res.failed > 0 {
		return fmt.Errorf("%d project(s) failed", res.failed)
	}
	return nil
}

// ── AES-128-CBC(key=iv,PKCS7,base64)——与前端 utils/crypto.ts 同规 ──

func encryptPassword(plain, key string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	padded := make([]byte, len(plain)+pad)
	copy(padded, plain)
	for i := len(plain); i < len(padded); i++ {
		padded[i] = byte(pad)
	}
	enc := cipher.NewCBCEncrypter(block, []byte(key)) // iv = key(同前端)
	out := make([]byte, len(padded))
	enc.CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out), nil
}

func truncate(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
