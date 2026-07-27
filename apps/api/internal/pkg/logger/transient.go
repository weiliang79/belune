package logger

import "strings"

// IsTransientConnError reports whether a log message describes a retryable
// network/connectivity failure (a dial that was refused, a name that did not yet
// resolve, a dropped connection) rather than a real fault.
//
// It exists for the boot-race window: on a full-stack restart or a host reboot,
// Docker starts belune in parallel with redis (restart policies do not honour
// depends_on), so the worker's first Redis calls fail with "connection refused"
// / "no such host" for a few seconds until redis is up, then recover on retry.
// asynq and go-redis both log those at Error/stderr, which makes a healthy boot
// look broken. Callers use this to downgrade such messages while leaving genuine
// errors untouched.
//
// Matched on message text because both libraries hand us a preformatted string,
// not a typed error. The substrings are the stable parts of Go's net-dial and
// DNS errors and go-redis's pool/pubsub teardown.
func IsTransientConnError(msg string) bool {
	m := strings.ToLower(msg)
	for _, s := range []string{
		"connection refused",
		"no such host",
		"i/o timeout",
		"connection reset by peer",
		"broken pipe",
		"discarding bad", // go-redis: "discarding bad PubSub connection: EOF"
	} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}
