package agent

import (
	"runtime"
	"testing"
)

func TestPathWithinDir(t *testing.T) {
	sep := "/"
	if runtime.GOOS == "windows" {
		sep = "\\"
	}
	base := sep + "srv" + sep + "app"
	cases := []struct {
		name   string
		base   string
		target string
		want   bool
	}{
		{"目录内常规文件", base, base + sep + "dist" + sep + "app.zip", true},
		{"目录本身", base, base, true},
		{"相对段逃逸", base, base + sep + ".." + sep + ".." + sep + "windows" + sep + "evil.dll", false},
		{"相对段折叠后回到目录内", base, base + sep + "a" + sep + ".." + sep + "b.txt", true},
		{"绝对路径外部", base, sep + "etc" + sep + "passwd", false},
		{"同名前缀目录不算内部", base, base + "-backup" + sep + "x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathWithinDir(tc.base, tc.target); got != tc.want {
				t.Fatalf("pathWithinDir(%q,%q)=%v want %v", tc.base, tc.target, got, tc.want)
			}
		})
	}
}
