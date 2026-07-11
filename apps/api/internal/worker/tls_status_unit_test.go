package worker

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsPublicIP(t *testing.T) {
	// A private or loopback autodetect must disable the DNS precheck rather than
	// flag every domain as misconfigured.
	assert.False(t, isPublicIP(net.ParseIP("192.168.1.10")))
	assert.False(t, isPublicIP(net.ParseIP("10.0.0.5")))
	assert.False(t, isPublicIP(net.ParseIP("127.0.0.1")))
	assert.True(t, isPublicIP(net.ParseIP("203.0.113.7")))
}
