package template

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// slugRe matches ids/names used as slugs and volume/db/service identifiers.
var slugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// inputKeyRe matches input keys (referenced as {{input.KEY}} and used as env-ish
// identifiers): letter-led, alphanumeric + underscore.
var inputKeyRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// Validate performs structural and referential validation of a manifest,
// returning all problems joined together. A manifest that passes is safe to
// resolve and instantiate.
func Validate(m *Manifest) error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if m.Schema != SchemaVersion {
		add("schema must be %d, got %d", SchemaVersion, m.Schema)
	}
	if !slugRe.MatchString(m.ID) {
		add("id %q must be a slug (lowercase alphanumeric and dashes)", m.ID)
	}
	if strings.TrimSpace(m.Name) == "" {
		add("name is required")
	}
	if strings.TrimSpace(m.Description) == "" {
		add("description is required")
	}
	if strings.TrimSpace(m.Category) == "" {
		add("category is required")
	}

	// Inputs: unique keys, valid validation rule.
	inputKeys := map[string]bool{}
	for i, in := range m.Inputs {
		if !inputKeyRe.MatchString(in.Key) {
			add("inputs[%d].key %q must be a letter-led identifier", i, in.Key)
			continue
		}
		if inputKeys[in.Key] {
			add("inputs[%d]: duplicate input key %q", i, in.Key)
		}
		inputKeys[in.Key] = true
		if strings.TrimSpace(in.Label) == "" {
			add("inputs[%d] (%s): label is required", i, in.Key)
		}
		if !validValidations[in.Validation] {
			add("inputs[%d] (%s): validation must be empty, email, or url", i, in.Key)
		}
	}

	// Databases: unique names, valid engine.
	dbNames := map[string]bool{}
	for i, db := range m.Databases {
		if !slugRe.MatchString(db.Name) {
			add("databases[%d].name %q must be a slug", i, db.Name)
			continue
		}
		if dbNames[db.Name] {
			add("databases[%d]: duplicate database name %q", i, db.Name)
		}
		dbNames[db.Name] = true
		if !validEngines[db.Engine] {
			add("databases[%d] (%s): engine must be one of postgres|mysql|redis|mongo|other", i, db.Name)
		}
	}

	// Services: at least one, unique names, image required, at least one routable.
	if len(m.Services) == 0 {
		add("at least one service is required")
	}
	serviceNames := map[string]bool{}
	anyPort := false
	for i, svc := range m.Services {
		if !slugRe.MatchString(svc.Name) {
			add("services[%d].name %q must be a slug", i, svc.Name)
		} else if serviceNames[svc.Name] {
			add("services[%d]: duplicate service name %q", i, svc.Name)
		}
		serviceNames[svc.Name] = true

		if strings.TrimSpace(svc.Image) == "" {
			add("services[%d] (%s): image is required", i, svc.Name)
		}
		if svc.Port < 0 || svc.Port > 65535 {
			add("services[%d] (%s): port %d out of range", i, svc.Name, svc.Port)
		}
		if svc.Port > 0 {
			anyPort = true
		}

		// Volumes: unique name + absolute, unique mount path per service.
		volNames := map[string]bool{}
		mountPaths := map[string]bool{}
		for j, v := range svc.Volumes {
			if !slugRe.MatchString(v.Name) {
				add("services[%d].volumes[%d].name %q must be a slug", i, j, v.Name)
			} else if volNames[v.Name] {
				add("services[%d] (%s): duplicate volume name %q", i, svc.Name, v.Name)
			}
			volNames[v.Name] = true
			if !path.IsAbs(v.MountPath) {
				add("services[%d] (%s): volume %q mount_path must be absolute", i, svc.Name, v.Name)
			} else if mountPaths[v.MountPath] {
				add("services[%d] (%s): duplicate mount_path %q", i, svc.Name, v.MountPath)
			}
			mountPaths[v.MountPath] = true
		}
	}
	if len(m.Services) > 0 && !anyPort {
		add("at least one service must declare a port for the domain to route to")
	}

	// depends_on: references must exist (service or database) and services must
	// not form a cycle. Databases have no depends_on, so only service edges can
	// cycle.
	for i, svc := range m.Services {
		for _, dep := range svc.DependsOn {
			if !serviceNames[dep] && !dbNames[dep] {
				add("services[%d] (%s): depends_on %q references no service or database", i, svc.Name, dep)
			}
		}
	}
	if cyc := findServiceCycle(m); cyc != "" {
		add("depends_on cycle among services: %s", cyc)
	}

	// Placeholder lint: every {{...}} in env values and notes must be well-formed
	// and reference declared inputs/databases.
	lintPlaceholders := func(where, s string) {
		for _, match := range placeholderRe.FindAllStringSubmatch(s, -1) {
			ph, err := parsePlaceholder(match[1])
			if err != nil {
				add("%s: %v", where, err)
				continue
			}
			switch ph.kind {
			case kindInput:
				if !inputKeys[ph.inputKey] {
					add("%s: {{input.%s}} references an undeclared input", where, ph.inputKey)
				}
			case kindDB:
				if !dbNames[ph.dbName] {
					add("%s: {{db.%s.%s}} references an undeclared database", where, ph.dbName, ph.dbField)
				}
			}
		}
	}
	for i, svc := range m.Services {
		for k, v := range svc.Env {
			lintPlaceholders(fmt.Sprintf("services[%d] (%s) env %s", i, svc.Name, k), v)
		}
	}
	lintPlaceholders("notes", m.Notes)

	return errors.Join(errs...)
}

// findServiceCycle returns a human-readable cycle path among service depends_on
// edges, or "" when the graph is acyclic. Edges to databases are ignored (DBs
// are leaves).
func findServiceCycle(m *Manifest) string {
	edges := map[string][]string{}
	isService := map[string]bool{}
	for _, s := range m.Services {
		isService[s.Name] = true
	}
	for _, s := range m.Services {
		for _, dep := range s.DependsOn {
			if isService[dep] {
				edges[s.Name] = append(edges[s.Name], dep)
			}
		}
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string

	var visit func(n string) string
	visit = func(n string) string {
		color[n] = gray
		stack = append(stack, n)
		for _, dep := range edges[n] {
			switch color[dep] {
			case gray:
				return strings.Join(append(stack, dep), " -> ")
			case white:
				if c := visit(dep); c != "" {
					return c
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[n] = black
		return ""
	}

	for _, s := range m.Services {
		if color[s.Name] == white {
			if c := visit(s.Name); c != "" {
				return c
			}
		}
	}
	return ""
}

// DeployOrder returns service names in an order that respects depends_on
// (dependencies first). It assumes the manifest passed Validate (acyclic).
func (m *Manifest) DeployOrder() []string {
	isService := map[string]bool{}
	for _, s := range m.Services {
		isService[s.Name] = true
	}
	edges := map[string][]string{}
	for _, s := range m.Services {
		for _, dep := range s.DependsOn {
			if isService[dep] {
				edges[s.Name] = append(edges[s.Name], dep)
			}
		}
	}

	visited := map[string]bool{}
	var order []string
	var visit func(n string)
	visit = func(n string) {
		if visited[n] {
			return
		}
		visited[n] = true
		for _, dep := range edges[n] {
			visit(dep)
		}
		order = append(order, n)
	}
	for _, s := range m.Services {
		visit(s.Name)
	}
	return order
}
