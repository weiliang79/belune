package eventwatcher

import (
	"testing"

	"github.com/weiliang79/belune/internal/runtime"
)

// ListContainers returns stopped containers too (All: true), and they keep
// their labels — so the state filter is the only thing preventing an exited
// container from being read as a live service.
func TestRunningIndex(t *testing.T) {
	containers := []runtime.ContainerInfo{
		{
			Name: "proj-app-up", Status: "running",
			Labels: map[string]string{labelApplicationID: "app-up"},
		},
		{
			// The regression: after a host or control-plane restart every
			// container sits in "exited" but still carries its labels.
			Name: "proj-app-down", Status: "exited",
			Labels: map[string]string{labelApplicationID: "app-down"},
		},
		{
			Name: "db-up", Status: "running",
			Labels: map[string]string{labelDatabaseID: "db-up"},
		},
		{
			Name: "db-down", Status: "exited",
			Labels: map[string]string{labelDatabaseID: "db-down"},
		},
		{
			// Created but never started is not up either.
			Name: "proj-app-created", Status: "created",
			Labels: map[string]string{labelApplicationID: "app-created"},
		},
		// Legacy database with no label, recognised by name.
		{Name: "legacy-db", Status: "running", Labels: map[string]string{}},
	}

	apps, dbs, names := runningIndex(containers)

	if got, ok := apps["app-up"]; !ok || got != "proj-app-up" {
		t.Errorf("running app missing or misnamed: %q, present=%v", got, ok)
	}
	for _, id := range []string{"app-down", "app-created"} {
		if _, ok := apps[id]; ok {
			t.Errorf("%s: a non-running container must not count as a live app", id)
		}
	}

	if !dbs["db-up"] {
		t.Error("running database should be indexed")
	}
	if dbs["db-down"] {
		t.Error("exited database must not count as running")
	}

	if !names["legacy-db"] {
		t.Error("running unlabelled container should be indexed by name")
	}
	if names["proj-app-down"] || names["db-down"] {
		t.Error("exited containers must not appear in the name index")
	}
}

// An empty host must yield empty (non-nil) sets, so reconcile treats every
// application as stopped rather than panicking or skipping the correction.
func TestRunningIndexEmpty(t *testing.T) {
	apps, dbs, names := runningIndex(nil)
	if apps == nil || dbs == nil || names == nil {
		t.Fatal("runningIndex must return initialised maps")
	}
	if len(apps) != 0 || len(dbs) != 0 || len(names) != 0 {
		t.Errorf("expected empty sets, got %d/%d/%d", len(apps), len(dbs), len(names))
	}
}
