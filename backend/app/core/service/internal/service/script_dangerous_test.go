package service

import "testing"

func TestFindDangerousPatterns(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantAt  map[string]bool // 期望命中的模式(其它命中不校验)
		wantN   int             // 期望命中数下限;0 表示必须零命中
	}{
		{"空内容", "", nil, 0},
		{"普通脚本", "echo hello\ndocker compose up -d\nsystemctl restart nginx", nil, 0},
		{"普通清理", "rm -rf /tmp/build-cache\nrm /var/log/app.log", nil, 0},
		{"rm 递归根", "rm -rf /", map[string]bool{"rm 递归根目录": true}, 1},
		{"rm 通配根", "rm -rf /*", map[string]bool{"rm 递归根目录": true}, 1},
		{"rm 根带尾参", "rm -fr / --no-preserve-root", map[string]bool{"rm 递归根目录": true}, 1},
		{"rm 子目录不算", "rm -rf /opt/app/logs", nil, 0},
		{"mkfs", "mkfs.ext4 /dev/sdb1", map[string]bool{"mkfs 格式化文件系统": true}, 1},
		{"dd 覆盘", "dd if=/dev/zero of=/dev/sda bs=1M", map[string]bool{"dd 覆写块设备": true}, 1},
		{"dd 写文件不算", "dd if=a.bin of=out.bin", nil, 0},
		{"重定向覆盘", "cat vuln.img > /dev/sdb", map[string]bool{"重定向覆写块设备": true}, 1},
		{"chmod 根", "chmod -R 777 /", map[string]bool{"chmod 777 根目录": true}, 1},
		{"chmod 项目目录不算", "chmod -R 777 /opt/app", nil, 0},
		{"shutdown", "shutdown -h now", map[string]bool{"关机 (shutdown/halt/poweroff)": true}, 1},
		{"reboot", "reboot", map[string]bool{"重启 (reboot)": true}, 1},
		{"windows format", "format D: /q /y", map[string]bool{"format 格式化磁盘": true}, 1},
		{"windows del 盘根", "del /f /s /q C:\\", map[string]bool{"del/rd 递归删盘根": true}, 1},
		{"windows del 子目录不算", "del /f /s /q C:\\temp\\cache", nil, 0},
		{"windows rd 盘根", "rd /s /q D:\\", map[string]bool{"del/rd 递归删盘根": true}, 1},
		{"diskpart", "echo list disk | diskpart", map[string]bool{"diskpart 磁盘分区操作": true}, 1},
		{"powershell 递归删盘根", "Remove-Item -Recurse -Force C:\\", map[string]bool{"PowerShell 递归删盘根": true}, 1},
		{"fork 炸弹", ":(){ :|:& };:", map[string]bool{"fork 炸弹": true}, 1},
		{"多命中去重保序", "reboot\nshutdown -h", map[string]bool{
			"重启 (reboot)":                true,
			"关机 (shutdown/halt/poweroff)": true,
		}, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findDangerousPatterns(tc.content)
			if tc.wantN == 0 {
				if len(got) != 0 {
					t.Fatalf("expect zero hits, got %v", got)
				}
				return
			}
			if len(got) < tc.wantN {
				t.Fatalf("expect >=%d hits, got %v", tc.wantN, got)
			}
			hitSet := make(map[string]struct{}, len(got))
			for _, h := range got {
				hitSet[h] = struct{}{}
			}
			for name, want := range tc.wantAt {
				if !want {
					continue
				}
				if _, ok := hitSet[name]; !ok {
					t.Fatalf("expect pattern %q in hits %v", name, got)
				}
			}
		})
	}
}
