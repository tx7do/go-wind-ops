package totp

import (
	"testing"
	"time"
)

// rfcSeed RFC 6238 附录 B 参考种子 "12345678901234567890" 的 Base32 形态。
const rfcSeed = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

// TestVerifyRFC6238Vectors RFC 6238 附录 B 官方测试向量(SHA-1/8位,本实现 6 位
// 取向量后 6 位;30s 步长向量 T 与代码同源)。
func TestVerifyRFC6238Vectors(t *testing.T) {
	// RFC 向量(8 位) → 截取后 6 位即本实现输出(RFC 也给出 6 位口径:后 6 位一致)
	cases := []struct {
		t    int64
		code string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	}
	key, err := b32.DecodeString(rfcSeed)
	if err != nil {
		t.Fatalf("decode seed: %v", err)
	}
	for _, c := range cases {
		if got := hotp(key, uint64(c.t)/30); got != c.code {
			t.Errorf("T=%d: got %s want %s", c.t, got, c.code)
		}
	}
}

// TestVerifyWindow ±1 窗口容差:上一步/当前步/下一步均通过,隔步拒绝。
func TestVerifyWindow(t *testing.T) {
	now := time.Unix(1111111111, 0)
	if !Verify(rfcSeed, "081804", now) {
		t.Error("current step should match")
	}
	prev := hotp(mustDecode(t, rfcSeed), uint64(now.Unix())/30-1)
	if !Verify(rfcSeed, prev, now) {
		t.Error("previous step should match within ±1 window")
	}
	far := hotp(mustDecode(t, rfcSeed), uint64(now.Unix())/30-2)
	if Verify(rfcSeed, far, now) {
		t.Error("step beyond ±1 window must not match")
	}
	if Verify(rfcSeed, "00000", now) {
		t.Error("wrong length code must not match")
	}
	if Verify("not-base32!!!", "123456", now) {
		t.Error("invalid secret must not match")
	}
}

// TestGenerateSecret 长度与唯一性。
func TestGenerateSecret(t *testing.T) {
	s1, err := GenerateSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	s2, _ := GenerateSecret()
	if len(s1) != 32 || s1 == s2 {
		t.Fatalf("secret len=%d unique=%v", len(s1), s1 != s2)
	}
}

// TestOtpauthURI URI 形态与参数。
func TestOtpauthURI(t *testing.T) {
	uri := OtpauthURI("GoWind Ops", "admin@example.com", "ABCD2345ABCD2345")
	want := "otpauth://totp/GoWind%20Ops:admin@example.com?secret=ABCD2345ABCD2345&issuer=GoWind+Ops&algorithm=SHA1&digits=6&period=30"
	if uri != want {
		t.Fatalf("uri = %s\nwant     %s", uri, want)
	}
}

func mustDecode(t *testing.T, s string) []byte {
	t.Helper()
	raw, err := b32.DecodeString(s)
	if err != nil {
		t.Fatalf("decode %s: %v", s, err)
	}
	return raw
}
