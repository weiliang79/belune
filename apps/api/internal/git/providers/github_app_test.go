package providers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func testAppKey(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return string(pemBytes), key
}

func TestAppJWT_SignsAndVerifies(t *testing.T) {
	pemStr, key := testAppKey(t)
	cfg := AppConfig{Provider: "github", AppID: "12345", PrivateKey: pemStr}

	tokenStr, err := appJWT(cfg)
	if err != nil {
		t.Fatalf("appJWT: %v", err)
	}

	parsed, err := jwt.Parse(tokenStr, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodRSA); !ok {
			t.Fatalf("unexpected signing method: %v", tok.Header["alg"])
		}
		return &key.PublicKey, nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("verify jwt: %v (valid=%v)", err, parsed != nil && parsed.Valid)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["iss"] != "12345" {
		t.Fatalf("unexpected issuer: %v", claims["iss"])
	}
}

func TestAppJWT_BadKey(t *testing.T) {
	cfg := AppConfig{Provider: "github", AppID: "1", PrivateKey: "not a pem"}
	if _, err := appJWT(cfg); err == nil {
		t.Fatal("expected error for invalid private key")
	}
}

func TestGitHubAppAuthURL(t *testing.T) {
	got := githubApp{}.AuthURL(AppConfig{AppSlug: "my-app"}, "", "st&te")
	if !strings.HasPrefix(got, "https://github.com/apps/my-app/installations/new?state=") {
		t.Fatalf("unexpected install url: %s", got)
	}
	if !strings.Contains(got, "st%26te") {
		t.Fatalf("state not escaped: %s", got)
	}
}
