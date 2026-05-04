package caddy

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew_UnixSocketTransport verifies that a unix:// admin URL routes the
// HTTP request to the configured filesystem socket. We bind a tiny HTTP
// server to the socket and assert it receives the call.
func TestNew_UnixSocketTransport(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "admin.sock")

	ln, err := net.Listen("unix", sock)
	require.NoError(t, err)
	defer ln.Close()

	mux := http.NewServeMux()
	called := make(chan string, 1)
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		called <- r.URL.Path
		_, _ = io.WriteString(w, "pong")
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	c := New("unix://" + sock)

	req, err := http.NewRequest("GET", c.adminURL+"/ping", nil)
	require.NoError(t, err)
	resp, err := c.httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	select {
	case path := <-called:
		assert.Equal(t, "/ping", path)
	default:
		t.Fatal("server was not called")
	}

	// The socket must actually exist and be a unix socket. Sanity check.
	info, err := os.Stat(sock)
	require.NoError(t, err)
	assert.NotEqual(t, 0, int(info.Mode()&os.ModeSocket))
}

// TestNew_TCPDefault verifies the legacy TCP path still produces a usable
// client whose adminURL is preserved verbatim.
func TestNew_TCPDefault(t *testing.T) {
	c := New("http://caddy:2019")
	assert.Equal(t, "http://caddy:2019", c.adminURL)
	assert.Nil(t, c.httpClient.Transport, "TCP mode should leave default transport")
}
