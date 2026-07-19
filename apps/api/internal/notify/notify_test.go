package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func sampleEvent() Event {
	return Event{
		Type:       EventDeploymentFailed,
		Title:      "Deploy failed",
		Body:       "web failed to build",
		Link:       "https://belune.example/apps/web",
		OccurredAt: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC),
	}
}

func TestSeverity(t *testing.T) {
	cases := map[string]Severity{
		EventDeploymentSucceeded:   SeverityOK,
		EventDeploymentFailed:      SeverityError,
		EventDatabaseBackupFailed:  SeverityError,
		EventDatabaseRestored:      SeverityOK,
		EventDatabaseRestoreFailed: SeverityError,
		EventTLSExpiring:           SeverityWarn,
		EventTLSExpired:            SeverityError,
		EventTLSFailed:             SeverityError,
		EventVolumeBackupFailed:    SeverityError,
		EventVolumeRestored:        SeverityOK,
		EventVolumeRestoreFailed:   SeverityError,
	}
	for typ, want := range cases {
		if got := (Event{Type: typ}).Severity(); got != want {
			t.Errorf("%s: severity = %q, want %q", typ, got, want)
		}
	}
}

// captureServer records the first request it receives.
type capture struct {
	body    []byte
	headers http.Header
	path    string
	query   string
}

func newCaptureServer(t *testing.T, status int) (*httptest.Server, *capture) {
	t.Helper()
	cap := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.body, _ = io.ReadAll(r.Body)
		cap.headers = r.Header.Clone()
		cap.path = r.URL.Path
		cap.query = r.URL.RawQuery
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, cap
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, b)
	}
	return m
}

func TestDiscordSend(t *testing.T) {
	srv, cap := newCaptureServer(t, http.StatusNoContent)
	p := discordProvider{client: srv.Client()}
	cfg, _ := json.Marshal(discordConfig{WebhookURL: srv.URL})

	if err := p.Send(context.Background(), cfg, sampleEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	body := decode(t, cap.body)
	embeds, ok := body["embeds"].([]any)
	if !ok || len(embeds) != 1 {
		t.Fatalf("expected one embed, got %v", body["embeds"])
	}
	embed := embeds[0].(map[string]any)
	if embed["color"].(float64) != float64(discordColorError) {
		t.Errorf("color = %v, want error color", embed["color"])
	}
	if embed["url"] != "https://belune.example/apps/web" {
		t.Errorf("url = %v", embed["url"])
	}
}

func TestSlackSend(t *testing.T) {
	srv, cap := newCaptureServer(t, http.StatusOK)
	p := slackProvider{client: srv.Client()}
	cfg, _ := json.Marshal(slackConfig{WebhookURL: srv.URL})

	if err := p.Send(context.Background(), cfg, sampleEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	body := decode(t, cap.body)
	att := body["attachments"].([]any)[0].(map[string]any)
	if att["color"] != slackColorError {
		t.Errorf("color = %v, want %v", att["color"], slackColorError)
	}
}

func TestTelegramSend(t *testing.T) {
	srv, cap := newCaptureServer(t, http.StatusOK)
	p := telegramProvider{client: srv.Client(), apiBase: srv.URL}
	cfg, _ := json.Marshal(telegramConfig{BotToken: "T0KEN", ChatID: "42"})

	if err := p.Send(context.Background(), cfg, sampleEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if cap.path != "/botT0KEN/sendMessage" {
		t.Errorf("path = %q", cap.path)
	}
	body := decode(t, cap.body)
	if body["chat_id"] != "42" || body["parse_mode"] != "HTML" {
		t.Errorf("unexpected body: %v", body)
	}
}

func TestWebhookSignature(t *testing.T) {
	srv, cap := newCaptureServer(t, http.StatusOK)
	p := webhookProvider{client: srv.Client()}
	cfg, _ := json.Marshal(webhookConfig{URL: srv.URL, Secret: "s3cr3t"})

	if err := p.Send(context.Background(), cfg, sampleEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	mac := hmac.New(sha256.New, []byte("s3cr3t"))
	mac.Write(cap.body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got := cap.headers.Get("X-Belune-Signature"); got != want {
		t.Errorf("signature = %q, want %q", got, want)
	}
	body := decode(t, cap.body)
	if body["type"] != EventDeploymentFailed || body["severity"] != string(SeverityError) {
		t.Errorf("unexpected payload: %v", body)
	}
}

func TestWebhookNoSecretNoSignature(t *testing.T) {
	srv, cap := newCaptureServer(t, http.StatusOK)
	p := webhookProvider{client: srv.Client()}
	cfg, _ := json.Marshal(webhookConfig{URL: srv.URL})

	if err := p.Send(context.Background(), cfg, sampleEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if sig := cap.headers.Get("X-Belune-Signature"); sig != "" {
		t.Errorf("expected no signature, got %q", sig)
	}
}

func TestNtfySend(t *testing.T) {
	srv, cap := newCaptureServer(t, http.StatusOK)
	p := ntfyProvider{client: srv.Client()}
	cfg, _ := json.Marshal(ntfyConfig{ServerURL: srv.URL, Topic: "alerts", AccessToken: "tok"})

	if err := p.Send(context.Background(), cfg, sampleEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if cap.path != "/alerts" {
		t.Errorf("path = %q", cap.path)
	}
	if cap.headers.Get("Authorization") != "Bearer tok" {
		t.Errorf("auth header = %q", cap.headers.Get("Authorization"))
	}
	if cap.headers.Get("Click") != "https://belune.example/apps/web" {
		t.Errorf("click header = %q", cap.headers.Get("Click"))
	}
	if string(cap.body) != "web failed to build" {
		t.Errorf("body = %q", cap.body)
	}
}

func TestGotifySend(t *testing.T) {
	srv, cap := newCaptureServer(t, http.StatusOK)
	p := gotifyProvider{client: srv.Client()}
	cfg, _ := json.Marshal(gotifyConfig{ServerURL: srv.URL, AppToken: "app"})

	if err := p.Send(context.Background(), cfg, sampleEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if cap.path != "/message" {
		t.Errorf("path = %q", cap.path)
	}
	if cap.headers.Get("X-Gotify-Key") != "app" {
		t.Errorf("gotify key = %q", cap.headers.Get("X-Gotify-Key"))
	}
	body := decode(t, cap.body)
	if body["priority"].(float64) != float64(gotifyPriority(SeverityError)) {
		t.Errorf("priority = %v", body["priority"])
	}
}

// fakeMailer records the messages an email channel would send.
type fakeMailer struct{ sent []MailMessage }

func (f *fakeMailer) Send(_ context.Context, msg MailMessage) error {
	f.sent = append(f.sent, msg)
	return nil
}

func TestEmailSend(t *testing.T) {
	m := &fakeMailer{}
	p := emailProvider{mailer: m}
	cfg, _ := json.Marshal(emailConfig{Recipients: []string{"a@x.io", " ", "b@x.io"}})

	if err := p.Send(context.Background(), cfg, sampleEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(m.sent) != 2 {
		t.Fatalf("expected 2 mails, got %d", len(m.sent))
	}
	if m.sent[0].Subject != "Deploy failed" {
		t.Errorf("subject = %q", m.sent[0].Subject)
	}
}

func TestValidateConfig(t *testing.T) {
	reg := NewRegistry(nil)
	cases := []struct {
		typ     string
		cfg     any
		wantErr bool
	}{
		{"discord", discordConfig{WebhookURL: "https://x"}, false},
		{"discord", discordConfig{}, true},
		{"telegram", telegramConfig{BotToken: "t", ChatID: "1"}, false},
		{"telegram", telegramConfig{BotToken: "t"}, true},
		{"ntfy", ntfyConfig{Topic: "t"}, false},
		{"ntfy", ntfyConfig{}, true},
		{"gotify", gotifyConfig{ServerURL: "https://x", AppToken: "a"}, false},
		{"gotify", gotifyConfig{ServerURL: "https://x"}, true},
		{"webhook", webhookConfig{URL: "https://x"}, false},
		{"webhook", webhookConfig{}, true},
		{"email", emailConfig{Recipients: []string{"a@x.io"}}, false},
		{"email", emailConfig{Recipients: []string{" "}}, true},
	}
	for _, c := range cases {
		p, ok := reg.Provider(c.typ)
		if !ok {
			t.Fatalf("no provider for %q", c.typ)
		}
		raw, _ := json.Marshal(c.cfg)
		err := p.ValidateConfig(raw)
		if (err != nil) != c.wantErr {
			t.Errorf("%s %+v: err = %v, wantErr = %v", c.typ, c.cfg, err, c.wantErr)
		}
	}
}

func TestSendPropagatesHTTPError(t *testing.T) {
	srv, _ := newCaptureServer(t, http.StatusInternalServerError)
	p := discordProvider{client: srv.Client()}
	cfg, _ := json.Marshal(discordConfig{WebhookURL: srv.URL})
	if err := p.Send(context.Background(), cfg, sampleEvent()); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestMaskConfig(t *testing.T) {
	raw := json.RawMessage(`{"webhook_url":"https://x","secret":"","recipients":["a@x.io"]}`)
	m := MaskConfig(raw)
	if !m["webhook_url"] || m["secret"] || !m["recipients"] {
		t.Errorf("mask = %v", m)
	}
}
