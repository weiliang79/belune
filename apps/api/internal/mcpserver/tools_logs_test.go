package mcpserver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripANSI(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		"colour":            {"\x1b[33mmise\x1b[0m \x1b[33mWARN\x1b[0m x", "mise WARN x"},
		"json escaped":      {`{"msg":"\u001b[31mred\u001b[0m"}`, `{"msg":"red"}`},
		"cursor and erase":  {"a\x1b[2K\x1b[1Gb", "ab"},
		"osc title":         {"\x1b]0;title\x07text", "text"},
		"osc hyperlink st":  {"\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\", "link"},
		"plain untouched":   {"no escapes [33m here", "no escapes [33m here"},
		"literal backslash": {`C:\users\u0041`, `C:\users\u0041`},
	} {
		t.Run(name, func(t *testing.T) { assert.Equal(t, tc.want, stripANSI(tc.in)) })
	}
}

func TestTailLog(t *testing.T) {
	assert.Equal(t, "a\nb", tailLog("a\nb\n", 5, 1000), "under the cap is returned whole")
	assert.Equal(t, "[... 2 earlier lines omitted]\nc\nd", tailLog("a\nb\nc\nd", 2, 1000))

	// Byte cap on top of the line cap: a huge single line must not defeat the bound.
	big := "first\n" + strings.Repeat("x", 500)
	out := tailLog(big, 10, 100)
	assert.LessOrEqual(t, len(out), 100+len("[... earlier output omitted]\n"))
	assert.True(t, strings.HasPrefix(out, "[... earlier output omitted]"))

	// A cut in the middle of a multi-byte rune yields valid UTF-8.
	out = tailLog(strings.Repeat("é", 100), 10, 51)
	assert.True(t, strings.ToValidUTF8(out, "?") == out)
}
