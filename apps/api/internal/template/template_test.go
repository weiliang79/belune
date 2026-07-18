package template

import (
	"strings"
	"testing"
)

// TestEmbeddedCatalogValidates is the community-contribution guard: a malformed
// or invalid manifest added to catalog/manifests fails CI here.
func TestEmbeddedCatalogValidates(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatalf("embedded catalog failed to load/validate: %v", err)
	}
	list := cat.List()
	if len(list) == 0 {
		t.Fatal("catalog is empty")
	}
	// Sorted by name, unique ids, and every manifest independently valid.
	seen := map[string]bool{}
	for _, m := range list {
		if seen[m.ID] {
			t.Errorf("duplicate id %q", m.ID)
		}
		seen[m.ID] = true
		if err := Validate(m); err != nil {
			t.Errorf("%s: %v", m.ID, err)
		}
	}
	for _, id := range []string{"uptime-kuma", "umami", "ghost"} {
		if _, ok := cat.Get(id); !ok {
			t.Errorf("expected fixture template %q in catalog", id)
		}
	}
}

func TestSchemaEmbedded(t *testing.T) {
	b, err := Schema()
	if err != nil {
		t.Fatalf("schema not embedded: %v", err)
	}
	if !strings.Contains(string(b), "App Template Manifest") {
		t.Error("schema.json did not embed expected content")
	}
}

// base returns a minimal valid manifest that individual failure cases mutate.
func base() *Manifest {
	return &Manifest{
		Schema:      1,
		ID:          "acme",
		Name:        "Acme",
		Description: "An app.",
		Category:    "misc",
		Services: []Service{
			{Name: "web", Image: "acme:1", Port: 8080},
		},
	}
}

func TestValidateFailures(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Manifest)
		wantSub string
	}{
		{"bad schema", func(m *Manifest) { m.Schema = 2 }, "schema must be 1"},
		{"bad id", func(m *Manifest) { m.ID = "Acme_App" }, "must be a slug"},
		{"missing name", func(m *Manifest) { m.Name = "" }, "name is required"},
		{"missing category", func(m *Manifest) { m.Category = "" }, "category is required"},
		{"no services", func(m *Manifest) { m.Services = nil }, "at least one service"},
		{"missing image", func(m *Manifest) { m.Services[0].Image = "" }, "image is required"},
		{"no routable port", func(m *Manifest) { m.Services[0].Port = 0 }, "must declare a port"},
		{"dup service", func(m *Manifest) {
			m.Services = append(m.Services, Service{Name: "web", Image: "x:1", Port: 81})
		}, "duplicate service name"},
		{"bad engine", func(m *Manifest) {
			m.Databases = []Database{{Name: "db", Engine: "cockroach"}}
		}, "engine must be one of"},
		{"dup db", func(m *Manifest) {
			m.Databases = []Database{{Name: "db", Engine: "postgres"}, {Name: "db", Engine: "mysql"}}
		}, "duplicate database name"},
		{"dup input", func(m *Manifest) {
			m.Inputs = []Input{{Key: "K", Label: "K"}, {Key: "K", Label: "K"}}
		}, "duplicate input key"},
		{"bad validation", func(m *Manifest) {
			m.Inputs = []Input{{Key: "K", Label: "K", Validation: "phone"}}
		}, "validation must be"},
		{"relative mount", func(m *Manifest) {
			m.Services[0].Volumes = []Volume{{Name: "data", MountPath: "rel/path"}}
		}, "must be absolute"},
		{"dup mount", func(m *Manifest) {
			m.Services[0].Volumes = []Volume{{Name: "a", MountPath: "/d"}, {Name: "b", MountPath: "/d"}}
		}, "duplicate mount_path"},
		{"unknown depends_on", func(m *Manifest) {
			m.Services[0].DependsOn = []string{"ghost"}
		}, "references no service or database"},
		{"depends_on cycle", func(m *Manifest) {
			m.Services = []Service{
				{Name: "a", Image: "a:1", Port: 1, DependsOn: []string{"b"}},
				{Name: "b", Image: "b:1", Port: 2, DependsOn: []string{"a"}},
			}
		}, "cycle"},
		{"bad placeholder", func(m *Manifest) {
			m.Services[0].Env = map[string]string{"X": "{{secret}}"}
		}, "secret placeholder"},
		{"undeclared input ref", func(m *Manifest) {
			m.Services[0].Env = map[string]string{"X": "{{input.missing}}"}
		}, "undeclared input"},
		{"undeclared db ref", func(m *Manifest) {
			m.Services[0].Env = map[string]string{"X": "{{db.nope.url}}"}
		}, "undeclared database"},
		{"bad db field", func(m *Manifest) {
			m.Databases = []Database{{Name: "db", Engine: "postgres"}}
			m.Services[0].Env = map[string]string{"X": "{{db.db.schema}}"}
		}, "db field must be"},
		{"health path not absolute", func(m *Manifest) {
			m.Services[0].HealthCheck = &HealthCheck{Path: "healthz"}
		}, "health_check.path must start with /"},
		{"health timeout out of range", func(m *Manifest) {
			m.Services[0].HealthCheck = &HealthCheck{Path: "/", TimeoutSeconds: 99999}
		}, "health_check.timeout_seconds"},
		{"health bad status", func(m *Manifest) {
			m.Services[0].HealthCheck = &HealthCheck{Path: "/", ExpectStatus: 42}
		}, "health_check.expect_status"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := base()
			tc.mutate(m)
			err := Validate(m)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantSub, err)
			}
		})
	}
}

func TestValidatePassesForValid(t *testing.T) {
	m := base()
	m.Inputs = []Input{{Key: "admin_email", Label: "Email", Validation: "email"}}
	m.Databases = []Database{{Name: "db", Engine: "postgres", Version: "16"}}
	m.Services[0].Env = map[string]string{
		"URL":    "{{domain.url}}",
		"DB":     "{{db.db.url}}",
		"HOST":   "{{db.db.host}}",
		"SECRET": "{{secret 32}}",
		"MAIL":   "{{input.admin_email}}",
	}
	m.Services[0].DependsOn = []string{"db"}
	m.Services[0].HealthCheck = &HealthCheck{Path: "/healthz", TimeoutSeconds: 300, ExpectStatus: 200}
	if err := Validate(m); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestResolve(t *testing.T) {
	ctx := ResolveContext{
		Inputs: map[string]string{"admin_email": "a@b.com"},
		Databases: map[string]DBConn{
			"db": {URL: "postgres://u:p@h:5432/d", Host: "h", Port: "5432", User: "u", Password: "p", Database: "d"},
		},
		Domain: DomainValues{URL: "https://x.example.com", Host: "x.example.com"},
	}
	cases := map[string]string{
		"{{input.admin_email}}": "a@b.com",
		"{{db.db.url}}":         "postgres://u:p@h:5432/d",
		"{{db.db.host}}":        "h",
		"{{db.db.port}}":        "5432",
		"{{db.db.user}}":        "u",
		"{{db.db.password}}":    "p",
		"{{db.db.database}}":    "d",
		"{{domain.url}}":        "https://x.example.com",
		"{{domain.host}}":       "x.example.com",
		"prefix {{db.db.host}}:{{db.db.port}} suffix": "prefix h:5432 suffix",
		"{{ domain.host }}":                           "x.example.com", // whitespace tolerant
	}
	for in, want := range cases {
		got, err := Resolve(in, ctx)
		if err != nil {
			t.Errorf("Resolve(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("Resolve(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveSecretLengthAndUniqueness(t *testing.T) {
	a, err := Resolve("{{secret 24}}", ResolveContext{})
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 24 {
		t.Errorf("secret length = %d, want 24", len(a))
	}
	b, _ := Resolve("{{secret 24}}", ResolveContext{})
	if a == b {
		t.Error("two secrets should differ")
	}
	// Two secrets in one string should also differ.
	pair, _ := Resolve("{{secret 16}}|{{secret 16}}", ResolveContext{})
	parts := strings.Split(pair, "|")
	if len(parts) == 2 && parts[0] == parts[1] {
		t.Error("secrets within one string should differ")
	}
}

func TestResolveMissingReference(t *testing.T) {
	if _, err := Resolve("{{input.nope}}", ResolveContext{}); err == nil {
		t.Error("expected error for missing input")
	}
	if _, err := Resolve("{{db.nope.url}}", ResolveContext{}); err == nil {
		t.Error("expected error for missing db")
	}
}

func TestDeployOrder(t *testing.T) {
	m := &Manifest{Services: []Service{
		{Name: "web", DependsOn: []string{"worker", "db"}},
		{Name: "worker", DependsOn: []string{"db"}},
		{Name: "db"}, // a service named db (not a managed DB) — ordering still holds
	}}
	order := m.DeployOrder()
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	if pos["db"] > pos["worker"] || pos["worker"] > pos["web"] {
		t.Errorf("bad deploy order: %v", order)
	}
}
