package service

import (
	"strings"
	"testing"
)

func TestRedactDetails_NilReturnsNil(t *testing.T) {
	if got := redactDetails(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestRedactDetails_MasksSensitiveKeys(t *testing.T) {
	in := map[string]any{
		"email":          "user@example.com",
		"password":       "hunter2",
		"webhook_secret": "abcdef",
		"api_key":        "sk_live_1234",
		"X-Hub-Signature": "sha256=...",
		"authorization":  "Bearer xyz",
		"cookie":         "session=foo",
		"token_hash":     []byte{1, 2, 3}, // value type irrelevant when key matches
	}

	out := redactDetails(in)

	for _, k := range []string{"password", "webhook_secret", "api_key", "X-Hub-Signature", "authorization", "cookie", "token_hash"} {
		if out[k] != "[REDACTED]" {
			t.Errorf("expected key %q to be redacted, got %v", k, out[k])
		}
	}
	if out["email"] != "user@example.com" {
		t.Errorf("expected non-sensitive key preserved, got %v", out["email"])
	}
}

func TestRedactDetails_ScrubsInlineCredsInStrings(t *testing.T) {
	in := map[string]any{
		"clone_url":       "https://x-token:ghp_foobar@github.com/me/repo.git",
		"error":           "Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig failed",
		"safe_message":    "deploy succeeded",
		"x-api-key":       "kept-as-key-mask", // value redacted by key match
	}

	out := redactDetails(in)

	if cu, _ := out["clone_url"].(string); !strings.Contains(cu, "[REDACTED]") || strings.Contains(cu, "ghp_foobar") {
		t.Errorf("clone_url not scrubbed: %v", out["clone_url"])
	}
	if e, _ := out["error"].(string); !strings.Contains(e, "[REDACTED]") || strings.Contains(e, "eyJhbGciOi") {
		t.Errorf("error string not scrubbed: %v", out["error"])
	}
	if out["safe_message"] != "deploy succeeded" {
		t.Errorf("safe_message altered: %v", out["safe_message"])
	}
	if out["x-api-key"] != "[REDACTED]" {
		t.Errorf("x-api-key not redacted: %v", out["x-api-key"])
	}
}

func TestRedactDetails_RecursesIntoNestedMaps(t *testing.T) {
	in := map[string]any{
		"meta": map[string]any{
			"password": "shhh",
			"branch":   "main",
		},
	}
	out := redactDetails(in)
	nested, ok := out["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %T", out["meta"])
	}
	if nested["password"] != "[REDACTED]" {
		t.Errorf("nested password not redacted: %v", nested["password"])
	}
	if nested["branch"] != "main" {
		t.Errorf("nested non-sensitive key altered: %v", nested["branch"])
	}
}

func TestRedactDetails_PreservesNonStringValues(t *testing.T) {
	in := map[string]any{
		"count":   42,
		"enabled": true,
		"nums":    []int{1, 2, 3},
	}
	out := redactDetails(in)
	if out["count"] != 42 || out["enabled"] != true {
		t.Errorf("non-string values altered: %v", out)
	}
}
