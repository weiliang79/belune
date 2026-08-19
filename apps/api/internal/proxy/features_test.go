package proxy_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weiliang79/belune/internal/proxy"
)

// TestParseFeatureConfig_RejectsRateLimit: rate_limit was accepted, stored, and
// then emitted no Caddy handler, so a domain looked rate-limited and was not.
// Rejecting is the honest answer until the custom Caddy image ships.
func TestParseFeatureConfig_RejectsRateLimit(t *testing.T) {
	_, err := proxy.ParseFeatureConfig(proxy.FeatureRateLimit, json.RawMessage(`{"rate":"10/s"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported in this release")
	assert.Contains(t, err.Error(), "0.2.0", "the message must say the feature is coming, not that it never will")
}

// The rejection must be targeted: the features the dashboard actually offers
// still have to parse.
func TestParseFeatureConfig_OfferedFeaturesStillParse(t *testing.T) {
	cases := map[string]string{
		proxy.FeatureBasicAuth:   `{"username":"u","hashed_password":"$2a$10$abcdefghijklmnopqrstuv"}`,
		proxy.FeatureHeaders:     `{"response":{"set":{"X-Frame-Options":["DENY"]}}}`,
		proxy.FeatureIPAllowlist: `{"ranges":["10.0.0.0/8"]}`,
		proxy.FeatureRedirect:    `{"from":"/old","to":"/new"}`,
	}
	for featureType, raw := range cases {
		t.Run(featureType, func(t *testing.T) {
			parsed, err := proxy.ParseFeatureConfig(featureType, json.RawMessage(raw))
			require.NoError(t, err)
			assert.NotNil(t, parsed)
		})
	}
}
