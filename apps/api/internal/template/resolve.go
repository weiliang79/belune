package template

import (
	"crypto/rand"
	"fmt"
)

// DBConn holds the resolved connection fields of a provisioned managed database,
// keyed into ResolveContext.Databases by the database's declared manifest name.
type DBConn struct {
	URL      string
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

// DomainValues holds the hostname the user chose in the wizard.
type DomainValues struct {
	URL  string // e.g. https://blog.example.com
	Host string // e.g. blog.example.com
}

// ResolveContext supplies the values placeholders resolve against. Secrets are
// generated on demand, so they need no context.
type ResolveContext struct {
	Inputs    map[string]string
	Databases map[string]DBConn
	Domain    DomainValues
}

// secretAlphabet is URL-safe and shell-safe: no quoting hazards in env values.
const secretAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// generateSecret returns a cryptographically random string of n characters.
func generateSecret(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	for i, b := range buf {
		buf[i] = secretAlphabet[int(b)%len(secretAlphabet)]
	}
	return string(buf), nil
}

// Resolve substitutes every placeholder in s using ctx. Each {{secret N}} yields
// a fresh random value. It assumes the manifest passed Validate, so referential
// lookups that miss are reported as errors rather than silently blanked.
func Resolve(s string, ctx ResolveContext) (string, error) {
	var resolveErr error
	out := placeholderRe.ReplaceAllStringFunc(s, func(match string) string {
		if resolveErr != nil {
			return match
		}
		inner := placeholderRe.FindStringSubmatch(match)[1]
		ph, err := parsePlaceholder(inner)
		if err != nil {
			resolveErr = err
			return match
		}
		val, err := resolveOne(ph, ctx)
		if err != nil {
			resolveErr = err
			return match
		}
		return val
	})
	if resolveErr != nil {
		return "", resolveErr
	}
	return out, nil
}

// UsesDomain reports whether any service env value or the notes reference a
// {{domain.*}} placeholder, i.e. the template requires a hostname to deploy
// correctly. The wizard uses this to make hostname required for such templates.
func (m *Manifest) UsesDomain() bool {
	scan := func(s string) bool {
		for _, match := range placeholderRe.FindAllStringSubmatch(s, -1) {
			if ph, err := parsePlaceholder(match[1]); err == nil && ph.kind == kindDomain {
				return true
			}
		}
		return false
	}
	for _, svc := range m.Services {
		for _, v := range svc.Env {
			if scan(v) {
				return true
			}
		}
	}
	return scan(m.Notes)
}

func resolveOne(ph placeholder, ctx ResolveContext) (string, error) {
	switch ph.kind {
	case kindSecret:
		return generateSecret(ph.secretLen)
	case kindInput:
		v, ok := ctx.Inputs[ph.inputKey]
		if !ok {
			return "", fmt.Errorf("no value for input %q", ph.inputKey)
		}
		return v, nil
	case kindDB:
		conn, ok := ctx.Databases[ph.dbName]
		if !ok {
			return "", fmt.Errorf("no connection for database %q", ph.dbName)
		}
		switch ph.dbField {
		case "url":
			return conn.URL, nil
		case "host":
			return conn.Host, nil
		case "port":
			return conn.Port, nil
		case "user":
			return conn.User, nil
		case "password":
			return conn.Password, nil
		case "database":
			return conn.Database, nil
		}
		return "", fmt.Errorf("unknown db field %q", ph.dbField)
	case kindDomain:
		switch ph.domainField {
		case "url":
			return ctx.Domain.URL, nil
		case "host":
			return ctx.Domain.Host, nil
		}
		return "", fmt.Errorf("unknown domain field %q", ph.domainField)
	}
	return "", fmt.Errorf("unhandled placeholder kind")
}
