package providers

import "strings"

// commitMessageLimit caps what we persist from a commit message. Messages are
// unbounded in git and this text is display-only, so we keep enough for a
// subject line plus a little body and drop the rest rather than let a webhook
// decide how much of our database it writes to.
const commitMessageLimit = 1000

// githubShapedPush is the push payload GitHub sends — and Gitea copies almost
// field for field, which is why both providers decode into this one struct.
// `head_commit` is GitHub-only; Gitea populates `commits` alone, so the commit
// lookup below falls back to matching `after` against the array.
type githubShapedPush struct {
	Ref        string         `json:"ref"`
	After      string         `json:"after"`
	HeadCommit *commitObject  `json:"head_commit"`
	Commits    []commitObject `json:"commits"`
	Repository struct {
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
}

// commitObject is one commit as GitHub, Gitea, and GitLab all report it. Only
// the display fields are decoded; the file lists (added/removed/modified) are
// deliberately ignored.
type commitObject struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Author  struct {
		Name     string `json:"name"`
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"author"`
}

// pickCommit returns the commit a push should be attributed to: the tip of the
// push. GitHub names it outright via head_commit; otherwise we match the `after`
// SHA against the commits array, and fall back to the last element (all three
// providers order commits oldest-first).
func pickCommit(head *commitObject, commits []commitObject, after string) *commitObject {
	if head != nil && head.ID != "" {
		return head
	}
	for i := range commits {
		if commits[i].ID == after && after != "" {
			return &commits[i]
		}
	}
	if len(commits) > 0 {
		return &commits[len(commits)-1]
	}
	return nil
}

// commitDetails flattens a commit into the message and author we store. The
// author falls back through name → username → email, since which of the three
// is populated varies by provider and by how the commit was created.
func commitDetails(c *commitObject) (message, author string) {
	if c == nil {
		return "", ""
	}
	message = truncateMessage(c.Message)
	switch {
	case c.Author.Name != "":
		author = c.Author.Name
	case c.Author.Username != "":
		author = c.Author.Username
	default:
		author = c.Author.Email
	}
	return message, author
}

// truncateMessage trims trailing whitespace and caps the message length,
// cutting on a rune boundary so a multi-byte character is never split.
func truncateMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if len(msg) <= commitMessageLimit {
		return msg
	}
	trimmed := msg[:commitMessageLimit]
	for len(trimmed) > 0 && !isRuneStart(msg[len(trimmed)]) {
		trimmed = trimmed[:len(trimmed)-1]
	}
	return strings.TrimSpace(trimmed) + "…"
}

// isRuneStart reports whether b begins a UTF-8 rune (i.e. is not a
// continuation byte 10xxxxxx).
func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
