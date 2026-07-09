package docker

import "testing"

func TestIsPlatformVolume(t *testing.T) {
	tests := []struct {
		name    string
		volName string
		labels  map[string]string
		want    bool
	}{
		{
			name:    "managed-by label",
			volName: "anything",
			labels:  map[string]string{labelManagedBy: labelValue},
			want:    true,
		},
		{
			name:    "belune-data label",
			volName: "anything",
			labels:  map[string]string{labelData: "true"},
			want:    true,
		},
		{
			name:    "belune-cache label",
			volName: "anything",
			labels:  map[string]string{labelCache: "true"},
			want:    true,
		},
		{
			name:    "unlabeled app data volume protected by name prefix",
			volName: "belune-vol-7e1d0c9a-data",
			labels:  nil,
			want:    true,
		},
		{
			name:    "unlabeled CNB cache protected by name prefix",
			volName: "belune-cnb-cache-7e1d0c9a",
			labels:  map[string]string{},
			want:    true,
		},
		{
			name:    "foreign dangling volume is prunable",
			volName: "some-random-buildkit-cache",
			labels:  map[string]string{},
			want:    false,
		},
		{
			name:    "anonymous hash volume is prunable",
			volName: "c1aadcf306992e3a4e78874546085422b258794c0a39a878d989b205901f6f36",
			labels:  nil,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPlatformVolume(tt.volName, tt.labels); got != tt.want {
				t.Errorf("isPlatformVolume(%q, %v) = %v, want %v", tt.volName, tt.labels, got, tt.want)
			}
		})
	}
}
