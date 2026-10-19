package server

import (
	"net/http"
	"testing"
)

func TestSameOriginWs(t *testing.T) {
	newReq := func(origin, host string) *http.Request {
		r, _ := http.NewRequest(http.MethodGet, "http://"+host+"/admin/v1/ws/terminal", nil)
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}

	cases := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{"无 Origin(非浏览器客户端)放行", "", "127.0.0.1:18600", true},
		{"同源", "http://127.0.0.1:18600", "127.0.0.1:18600", true},
		{"同源大小写不敏感", "http://LOCALhost:18600", "localhost:18600", true},
		{"跨站拒绝", "http://evil.example.com", "127.0.0.1:18600", false},
		{"同主机不同端口拒绝", "http://127.0.0.1:18999", "127.0.0.1:18600", false},
		{"Origin 解析失败拒绝", "::::", "127.0.0.1:18600", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameOriginWs(newReq(tc.origin, tc.host)); got != tc.want {
				t.Fatalf("sameOriginWs(origin=%q host=%q) = %v, want %v", tc.origin, tc.host, got, tc.want)
			}
		})
	}
}
