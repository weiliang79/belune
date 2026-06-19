package worker

import (
	"strings"
	"testing"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

func TestDBDumpSpec(t *testing.T) {
	creds := map[string]string{
		"user": "u", "password": "p", "database": "d", "username": "mu",
	}
	cases := []struct {
		dbType         string
		ok             bool
		dumpContains   string
		restoreContain string
	}{
		{"postgres", true, "pg_dump", "psql"},
		{"mysql", true, "mysqldump --single-transaction", "mysql -u"},
		{"mongo", true, "mongodump", "mongorestore"},
		{"redis", false, "", ""},
		{"other", false, "", ""},
	}
	for _, c := range cases {
		t.Run(c.dbType, func(t *testing.T) {
			spec := dbDumpSpec(c.dbType, creds)
			if spec.ok != c.ok {
				t.Fatalf("ok = %v, want %v", spec.ok, c.ok)
			}
			if !c.ok {
				return
			}
			dump := strings.Join(spec.dump, " ")
			restore := strings.Join(spec.restore, " ")
			if !strings.Contains(dump, c.dumpContains) {
				t.Errorf("dump %q missing %q", dump, c.dumpContains)
			}
			if !strings.Contains(restore, c.restoreContain) {
				t.Errorf("restore %q missing %q", restore, c.restoreContain)
			}
		})
	}
}

func TestShArgEscapesQuotes(t *testing.T) {
	// A password with a single quote must be escaped so it cannot break out of
	// the surrounding single-quoted shell argument.
	got := shArg("p'wd")
	want := `'p'\''wd'`
	if got != want {
		t.Fatalf("shArg = %q, want %q", got, want)
	}
}

func TestDBBackupMethod(t *testing.T) {
	cases := []struct {
		dbType     string
		backupMode string
		want       string
	}{
		{"postgres", "none", "logical"},
		{"mysql", "none", "logical"},
		{"mongo", "none", "logical"},
		{"redis", "none", "none"},
		{"other", "volume_snapshot", "volume_snapshot"},
		{"other", "command", "command"},
		{"other", "none", "none"},
	}
	for _, c := range cases {
		got := dbBackupMethod(generated.Database{Type: c.dbType, BackupMode: c.backupMode})
		if got != c.want {
			t.Errorf("dbBackupMethod(%s,%s) = %q, want %q", c.dbType, c.backupMode, got, c.want)
		}
	}
}
