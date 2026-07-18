package template

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// placeholderRe matches a single {{ ... }} placeholder, capturing the inner
// token. Inner whitespace is tolerated: {{secret 32}} and {{ secret 32 }} are
// equivalent. The inner group is non-greedy and forbids braces so adjacent
// placeholders don't merge.
var placeholderRe = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

// maxSecretLen caps {{secret N}} so a malformed manifest can't request a
// pathologically large allocation. Well beyond any real secret length.
const maxSecretLen = 4096

// placeholderKind enumerates the closed set of placeholder forms.
type placeholderKind int

const (
	kindSecret placeholderKind = iota
	kindInput
	kindDB
	kindDomain
)

// placeholder is a parsed {{ ... }} token.
type placeholder struct {
	kind placeholderKind
	// secret
	secretLen int
	// input
	inputKey string
	// db
	dbName  string
	dbField string
	// domain
	domainField string
}

var (
	dbFields     = map[string]bool{"url": true, "host": true, "port": true, "user": true, "password": true, "database": true}
	domainFields = map[string]bool{"url": true, "host": true}
)

// parsePlaceholder parses the inner token of a {{ ... }} placeholder. It does
// NOT check referential validity (that an input/db exists) — the validator does
// that with manifest context.
func parsePlaceholder(inner string) (placeholder, error) {
	fields := strings.Fields(inner)
	if len(fields) == 0 {
		return placeholder{}, fmt.Errorf("empty placeholder")
	}

	// {{secret N}}
	if fields[0] == "secret" {
		if len(fields) != 2 {
			return placeholder{}, fmt.Errorf("secret placeholder must be {{secret N}}")
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil || n <= 0 {
			return placeholder{}, fmt.Errorf("secret length must be a positive integer, got %q", fields[1])
		}
		if n > maxSecretLen {
			return placeholder{}, fmt.Errorf("secret length must be at most %d, got %d", maxSecretLen, n)
		}
		return placeholder{kind: kindSecret, secretLen: n}, nil
	}

	// The remaining forms are dotted single tokens with no spaces.
	if len(fields) != 1 {
		return placeholder{}, fmt.Errorf("unknown placeholder %q", inner)
	}
	parts := strings.Split(fields[0], ".")
	switch parts[0] {
	case "input":
		if len(parts) != 2 || parts[1] == "" {
			return placeholder{}, fmt.Errorf("input placeholder must be {{input.KEY}}")
		}
		return placeholder{kind: kindInput, inputKey: parts[1]}, nil
	case "db":
		if len(parts) != 3 || parts[1] == "" {
			return placeholder{}, fmt.Errorf("db placeholder must be {{db.NAME.FIELD}}")
		}
		if !dbFields[parts[2]] {
			return placeholder{}, fmt.Errorf("db field must be one of url|host|port|user|password|database, got %q", parts[2])
		}
		return placeholder{kind: kindDB, dbName: parts[1], dbField: parts[2]}, nil
	case "domain":
		if len(parts) != 2 || !domainFields[parts[1]] {
			return placeholder{}, fmt.Errorf("domain placeholder must be {{domain.url}} or {{domain.host}}")
		}
		return placeholder{kind: kindDomain, domainField: parts[1]}, nil
	default:
		return placeholder{}, fmt.Errorf("unknown placeholder %q", inner)
	}
}
