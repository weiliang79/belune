package handler

// The integration tests seed the mock runtime directly and share one Handler
// across tests, so a cached container read would leak one test's helper into
// the next. Disabling the TTL keeps every request honest; the cache itself is
// covered by update_probe_test.go.
func init() { updateProbeTTL = 0 }
