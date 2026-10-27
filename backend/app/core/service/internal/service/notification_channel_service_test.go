package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var signTestNow = time.Unix(1700000000, 0)

// TestBuildWebhookBody_Styles 三种群机器人消息体形态与通用兜底。
func TestBuildWebhookBody_Styles(t *testing.T) {
	cases := []struct {
		style    string
		wantKey  string // 顶层消息类型键
		wantText string // 消息体文本内容
	}{
		// JSON 体内换行序列化为字面反斜杠 n
		{"DINGTALK", "msgtype", `标题\n内容`},
		{"WECOM", "msgtype", `标题\n内容`},
		{"FEISHU", "msg_type", `标题\n内容`},
	}
	for _, c := range cases {
		body, _ := buildWebhookBody("https://example.com/hook", c.style, "", "标题", "内容", signTestNow)
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("%s: unmarshal: %v", c.style, err)
		}
		if _, ok := m[c.wantKey]; !ok {
			t.Errorf("%s: missing key %s in %s", c.style, c.wantKey, body)
		}
		if !strings.Contains(string(body), c.wantText) {
			t.Errorf("%s: body %s missing text", c.style, body)
		}
	}

	// 通用兜底:无加签字段
	body, _ := buildWebhookBody("https://example.com", "NONE", "", "t", "c", signTestNow)
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	if m["title"] != "t" || m["content"] != "c" {
		t.Errorf("generic body = %s", body)
	}
	if _, has := m["sign"]; has {
		t.Errorf("NONE style must not carry sign")
	}
}

// TestBuildWebhookBody_DingtalkSign 加签:URL 追加 timestamp+sign,毫秒时间戳。
func TestBuildWebhookBody_DingtalkSign(t *testing.T) {
	_, withQuery := buildWebhookBody("https://oapi.dingtalk.com/robot/send?access_token=k",
		"DINGTALK", "sec", "t", "c", signTestNow)
	if !strings.Contains(withQuery, "timestamp=1700000000000") || !strings.Contains(withQuery, "&sign=") {
		t.Errorf("dingtalk signed URL = %s", withQuery)
	}

	// URL 已含 query 时用 & 追加;否则 ? 起
	body, plain := buildWebhookBody("https://oapi.dingtalk.com/robot/send",
		"DINGTALK", "sec", "t", "c", signTestNow)
	if !strings.Contains(plain, "?timestamp=") || strings.Contains(string(body), "timestamp") {
		t.Errorf("dingtalk: url=%s body=%s", plain, body)
	}
}

// TestBuildWebhookBody_FeishuSign 加签:body 注入秒级 timestamp 与 sign。
func TestBuildWebhookBody_FeishuSign(t *testing.T) {
	body, _ := buildWebhookBody("https://open.feishu.cn/hook/x", "FEISHU", "sec", "t", "c", signTestNow)
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["timestamp"] != "1700000000" {
		t.Errorf("timestamp = %v", m["timestamp"])
	}
	if sign, _ := m["sign"].(string); sign == "" {
		t.Errorf("sign missing in %s", body)
	}

	// secret 为空不加签
	body2, _ := buildWebhookBody("https://open.feishu.cn/hook/x", "FEISHU", "", "t", "c", signTestNow)
	if strings.Contains(string(body2), "\"sign\"") {
		t.Errorf("unsigned feishu body = %s", body2)
	}
}
