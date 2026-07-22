package logger

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// ConsoleTimeFormat is the timestamp layout written by ConsoleHandler:
// 24-hour, no timezone suffix, second precision.
const ConsoleTimeFormat = "2006-01-02 15:04:05"

// levelColumn is the fixed width of the level field ("DEBUG"/"ERROR" are the
// longest). Padding keeps the [module] column aligned down the page.
const levelColumn = 5

// continuationIndent aligns the attribute line under the message column:
// len(timestamp) + space + level + space.
var continuationIndent = strings.Repeat(" ", len(ConsoleTimeFormat)+1+levelColumn+1)

// ConsoleHandler writes human-readable log lines:
//
//	2026-07-22 10:00:04 INFO  [worker.deploy] deploy finished
//	                          app_id=10d7ec5f dur=4.2s
//
// It exists because the JSON handler is unreadable in a terminal. JSON is still
// the right choice when something machine-parses the stream, so the format is
// selected by LOG_FORMAT rather than replaced outright.
//
// The [module] field is derived from the call site rather than passed in by
// callers: there are 500+ slog calls in this tree and none carry a component
// attribute, so anything requiring call-site cooperation would print an empty
// context almost everywhere and silently rot as new code is added.
//
// Anything changing this layout must also update:
//   - loglevel.Detect (Go), which levels container output, and
//   - parseLine in apps/web/src/components/logs/parse.ts, which levels the
//     platform log viewer.
//
// Both match on the "<date> <time> <LEVEL>" prefix. Without them every line
// here renders as Info, errors included.
type ConsoleHandler struct {
	opts   slog.HandlerOptions
	out    io.Writer
	mu     *sync.Mutex
	attrs  []slog.Attr
	groups []string
}

// NewConsoleHandler returns a handler writing to w. A nil opts means Info.
func NewConsoleHandler(w io.Writer, opts *slog.HandlerOptions) *ConsoleHandler {
	h := &ConsoleHandler{out: w, mu: &sync.Mutex{}}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo
	}
	return h
}

func (h *ConsoleHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.opts.Level.Level()
}

func (h *ConsoleHandler) WithAttrs(as []slog.Attr) slog.Handler {
	if len(as) == 0 {
		return h
	}
	n := h.clone()
	n.attrs = append(n.attrs, as...)
	return n
}

func (h *ConsoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	n := h.clone()
	n.groups = append(n.groups, name)
	return n
}

// clone copies the slices with their capacity clipped. Without the clip a
// later append on the parent could write into a child's backing array, so two
// loggers derived from the same parent would corrupt each other's attributes.
func (h *ConsoleHandler) clone() *ConsoleHandler {
	return &ConsoleHandler{
		opts:   h.opts,
		out:    h.out,
		mu:     h.mu,
		attrs:  h.attrs[:len(h.attrs):len(h.attrs)],
		groups: h.groups[:len(h.groups):len(h.groups)],
	}
}

func (h *ConsoleHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder

	b.WriteString(r.Time.Format(ConsoleTimeFormat))
	b.WriteByte(' ')
	b.WriteString(levelLabel(r.Level))
	b.WriteByte(' ')
	b.WriteByte('[')
	b.WriteString(ModuleFor(r.PC))
	b.WriteString("] ")
	b.WriteString(r.Message)

	prefix := ""
	if len(h.groups) > 0 {
		prefix = strings.Join(h.groups, ".") + "."
	}
	var kv []string
	for _, a := range h.attrs {
		kv = appendAttr(kv, prefix, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		kv = appendAttr(kv, prefix, a)
		return true
	})
	if len(kv) > 0 {
		b.WriteByte('\n')
		b.WriteString(continuationIndent)
		b.WriteString(strings.Join(kv, " "))
	}
	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.out, b.String())
	return err
}

// appendAttr flattens one attribute into "key=value" pairs, recursing through
// groups so a nested attr reads as "outer.inner=value".
func appendAttr(dst []string, prefix string, a slog.Attr) []string {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return dst
	}
	if a.Value.Kind() == slog.KindGroup {
		gs := a.Value.Group()
		if len(gs) == 0 {
			return dst
		}
		inner := prefix
		if a.Key != "" {
			inner = prefix + a.Key + "."
		}
		for _, g := range gs {
			dst = appendAttr(dst, inner, g)
		}
		return dst
	}
	return append(dst, prefix+a.Key+"="+formatValue(a.Value))
}

// formatValue quotes anything that would otherwise break "key=value" parsing.
func formatValue(v slog.Value) string {
	s := v.String()
	if s == "" || strings.ContainsAny(s, " \t\n\"") {
		return strconv.Quote(s)
	}
	return s
}

func levelLabel(l slog.Level) string {
	var s string
	switch {
	case l < slog.LevelInfo:
		s = "DEBUG"
	case l < slog.LevelWarn:
		s = "INFO"
	case l < slog.LevelError:
		s = "WARN"
	default:
		s = "ERROR"
	}
	if len(s) < levelColumn {
		s += strings.Repeat(" ", levelColumn-len(s))
	}
	return s
}

// moduleCache memoises PC -> module. Every call site resolves to the same
// module forever, so the frame lookup is done once rather than per line.
var moduleCache sync.Map // uintptr -> string

// ModuleFor resolves a record's program counter to a short module label.
func ModuleFor(pc uintptr) string {
	if pc == 0 {
		return "app"
	}
	if v, ok := moduleCache.Load(pc); ok {
		return v.(string)
	}
	f, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	m := moduleFromFile(f.File)
	moduleCache.Store(pc, m)
	return m
}

// moduleFromFile turns a source path into "<dir>.<file>":
//
//	internal/worker/deploy_task.go -> worker.deploy
//	internal/handler/webhooks.go   -> handler.webhooks
//	internal/logcollector/collector.go -> logcollector
//
// The file half is dropped when it only repeats the directory, so packages
// named after their main file do not stutter. "_task" is trimmed because every
// file in internal/worker carries it and it distinguishes nothing.
func moduleFromFile(file string) string {
	if file == "" {
		return "app"
	}
	dir := filepath.Base(filepath.Dir(file))
	stem := strings.TrimSuffix(filepath.Base(file), ".go")
	stem = strings.TrimSuffix(stem, "_task")
	switch {
	case dir == "" || dir == "." || dir == string(filepath.Separator):
		return "app"
	case stem == "" || stem == dir,
		strings.HasPrefix(dir, stem),
		strings.HasSuffix(dir, stem):
		return dir
	}
	return dir + "." + stem
}
