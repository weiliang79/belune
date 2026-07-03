package handler

import "testing"

func TestValidateMountPath(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid data dir", "/data", false},
		{"valid nested", "/var/lib/app/data", false},
		{"empty", "", true},
		{"relative", "data", true},
		{"relative dot", "./data", true},
		{"trailing slash", "/data/", true},
		{"double slash", "/data//x", true},
		{"parent traversal", "/data/../etc", true},
		{"root", "/", true},
		{"reserved tmp", "/tmp", true},
		{"reserved run", "/run", true},
		{"reserved proc", "/proc", true},
		{"reserved etc", "/etc", true},
		{"under proc", "/proc/self", true},
		{"under sys", "/sys/kernel", true},
		{"under dev", "/dev/shm", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMountPath(tc.path)
			if tc.wantErr && err == nil {
				t.Errorf("validateMountPath(%q) = nil, want error", tc.path)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateMountPath(%q) = %v, want nil", tc.path, err)
			}
		})
	}
}
