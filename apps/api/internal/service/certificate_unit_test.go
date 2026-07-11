package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// selfSigned mints a certificate/key PEM pair for testing. notAfter in the past
// produces an expired certificate; empty dnsNames produces one with no SANs.
func selfSigned(t *testing.T, dnsNames []string, notBefore, notAfter time.Time) (certPEM, keyPEM string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "belune-test"},
		Issuer:       pkix.Name{CommonName: "belune-test-ca"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		DNSNames:     dnsNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return certPEM, keyPEM
}

func TestParseCertificatePair(t *testing.T) {
	now := time.Now()
	goodCert, goodKey := selfSigned(t, []string{"app.example.com", "www.example.com"}, now.Add(-time.Hour), now.Add(24*time.Hour))
	expiredCert, expiredKey := selfSigned(t, []string{"old.example.com"}, now.Add(-48*time.Hour), now.Add(-24*time.Hour))
	noSANCert, noSANKey := selfSigned(t, nil, now.Add(-time.Hour), now.Add(24*time.Hour))
	_, otherKey := selfSigned(t, []string{"app.example.com"}, now.Add(-time.Hour), now.Add(24*time.Hour))

	tests := []struct {
		name    string
		cert    string
		key     string
		wantErr string
	}{
		{name: "valid pair", cert: goodCert, key: goodKey},
		{
			// Expiry is surfaced through not_after (and the TLS status pipeline),
			// not by refusing the upload: an operator replacing a cert that expired
			// an hour ago must still be able to load its successor.
			name: "expired certificate is accepted",
			cert: expiredCert, key: expiredKey,
		},
		{
			name: "key does not match certificate",
			cert: goodCert, key: otherKey,
			wantErr: "do not form a valid pair",
		},
		{
			name: "certificate without SANs",
			cert: noSANCert, key: noSANKey,
			wantErr: "no subject alternative names",
		},
		{
			name: "garbage PEM",
			cert: "not a certificate", key: goodKey,
			wantErr: "do not form a valid pair",
		},
		{
			name: "empty input",
			cert: "", key: "",
			wantErr: "both required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := parseCertificatePair(tc.cert, tc.key)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, parsed.subjects)
			assert.NotEmpty(t, parsed.issuer)
			assert.False(t, parsed.notAfter.IsZero())
		})
	}
}

func TestParseCertificatePair_ExtractsMetadata(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	certPEM, keyPEM := selfSigned(t, []string{"app.example.com", "www.example.com"}, now.Add(-time.Hour), now.Add(72*time.Hour))

	parsed, err := parseCertificatePair(certPEM, keyPEM)
	require.NoError(t, err)

	assert.Equal(t, []string{"app.example.com", "www.example.com"}, parsed.subjects)
	// Self-signed, so the issuer is the certificate's own subject.
	assert.Contains(t, parsed.issuer, "belune-test")
	assert.WithinDuration(t, now.Add(72*time.Hour), parsed.notAfter, time.Minute)
}

func TestLooksLikePEM(t *testing.T) {
	certPEM, _ := selfSigned(t, []string{"app.example.com"}, time.Now(), time.Now().Add(time.Hour))

	assert.True(t, looksLikePEM(certPEM))
	assert.False(t, looksLikePEM("/etc/ssl/certs/app.pem"), "a file path is the mistake the old cert_path field invited")
	assert.False(t, looksLikePEM(""))
}
