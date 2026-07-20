package handler

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestValidateHealthCheck(t *testing.T) {
	var id pgtype.UUID

	t.Run("valid", func(t *testing.T) {
		cases := map[string]healthCheckRequest{
			"none":              {Type: "none"},
			"http with path":    {Type: "http", Path: "/healthz"},
			"http with status":  {Type: "http", Path: "/healthz", ExpectStatus: 204},
			"command":           {Type: "command", Command: "pg_isready"},
			"command with tune": {Type: "command", Command: "curl -f localhost", IntervalSeconds: 10, RetriesCount: 2},
		}
		for name, req := range cases {
			t.Run(name, func(t *testing.T) {
				if _, err := validateHealthCheck(id, req); err != nil {
					t.Errorf("validateHealthCheck() = %v, want nil", err)
				}
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		cases := map[string]healthCheckRequest{
			"http with no path":    {Type: "http"},
			"http path not rooted": {Type: "http", Path: "healthz"},
			"http bad status":      {Type: "http", Path: "/x", ExpectStatus: 99},
			"command with no cmd":  {Type: "command"},
			"unknown type":         {Type: "tcp"},
			"empty type":           {Type: ""},
		}
		for name, req := range cases {
			t.Run(name, func(t *testing.T) {
				if _, err := validateHealthCheck(id, req); err == nil {
					t.Errorf("validateHealthCheck() = nil, want an error")
				}
			})
		}
	})

	t.Run("clears the unselected mechanism", func(t *testing.T) {
		// Switching to command must not leave a stale http path, or a later
		// deploy would run a check the UI no longer shows.
		p, err := validateHealthCheck(id, healthCheckRequest{Type: "command", Command: "pg_isready"})
		if err != nil {
			t.Fatal(err)
		}
		if p.HealthCheckPath.Valid {
			t.Error("command config must clear the http path")
		}
		if !p.HealthCheckCommand.Valid {
			t.Error("command must be set")
		}

		// ...and the reverse.
		p, err = validateHealthCheck(id, healthCheckRequest{Type: "http", Path: "/healthz"})
		if err != nil {
			t.Fatal(err)
		}
		if p.HealthCheckCommand.Valid {
			t.Error("http config must clear the command")
		}
	})
}
