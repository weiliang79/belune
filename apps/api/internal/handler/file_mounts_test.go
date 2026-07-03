package handler

import "testing"

func TestValidateFilePath(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// Valid: deep config-file paths, including under system dirs (this is the
		// key difference from volume validateMountPath, which bans /etc etc.).
		{"etc nginx conf", "/etc/nginx/nginx.conf", false},
		{"app config", "/app/config/config.yaml", false},
		{"usr share file", "/usr/local/share/app.json", false},
		{"nested deep", "/etc/app/conf.d/app.conf", false},

		{"empty", "", true},
		{"relative", "etc/app.conf", true},
		{"trailing slash", "/etc/nginx/", true},
		{"double slash", "/etc//nginx.conf", true},
		{"parent traversal", "/etc/../app.conf", true},
		{"root", "/", true},
		{"bare top-level (no dir)", "/app.conf", true},
		{"under proc", "/proc/1/config", true},
		{"under sys", "/sys/kernel/x.conf", true},
		{"under dev", "/dev/shm/x.conf", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFilePath(tc.path)
			if tc.wantErr && err == nil {
				t.Errorf("validateFilePath(%q) = nil, want error", tc.path)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateFilePath(%q) = %v, want nil", tc.path, err)
			}
		})
	}
}

func TestValidateFileMode(t *testing.T) {
	cases := []struct {
		mode    string
		want    string
		wantErr bool
	}{
		{"", "0644", false},
		{"0644", "0644", false},
		{"600", "600", false},
		{"0600", "0600", false},
		{"999", "", true}, // 9 is not an octal digit
		{"abc", "", true},
		{"0x10", "", true},
		{"4755", "", true}, // setuid bit rejected
		{"2755", "", true}, // setgid bit rejected
		{"1777", "", true}, // sticky bit rejected
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			got, err := validateFileMode(tc.mode)
			if tc.wantErr {
				if err == nil {
					t.Errorf("validateFileMode(%q) = nil error, want error", tc.mode)
				}
				return
			}
			if err != nil {
				t.Errorf("validateFileMode(%q) unexpected error: %v", tc.mode, err)
			}
			if got != tc.want {
				t.Errorf("validateFileMode(%q) = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}
