package backup

import "testing"

func TestBuildKey(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		file   string
		want   string
	}{
		{"empty prefix", "", "db.backup.gz", "db.backup.gz"},
		{"plain prefix", "prod", "db.backup.gz", "prod/db.backup.gz"},
		{"leading slash trimmed", "/medico-production", "db.gz", "medico-production/db.gz"},
		{"trailing slash trimmed", "prod/", "db.gz", "prod/db.gz"},
		{"both slashes trimmed", "/prod/", "db.gz", "prod/db.gz"},
		{"nested prefix", "team/prod", "db.gz", "team/prod/db.gz"},
		{"slash-only prefix", "/", "db.gz", "db.gz"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BuildKey(c.prefix, c.file); got != c.want {
				t.Fatalf("BuildKey(%q, %q) = %q, want %q", c.prefix, c.file, got, c.want)
			}
		})
	}
}
