package notify

import "encoding/json"

// secretConfigFields lists, per channel type, the top-level config keys whose
// values are secrets: they are stripped before config is returned to a client,
// and on update a blank submitted value preserves the stored one. Non-secret
// connection fields (topics, chat ids, URLs, recipients) are returned as-is so
// the edit form can prefill them.
//
// The email provider's secret is nested at smtp.password and handled specially.
var secretConfigFields = map[string][]string{
	"discord":  {"webhook_url"},
	"slack":    {"webhook_url"},
	"telegram": {"bot_token"},
	"ntfy":     {"access_token"},
	"gotify":   {"app_token"},
	"webhook":  {"secret"},
	"email":    {},
}

// RedactConfig returns a copy of the decrypted config with every secret value
// removed — safe to serialise to the admin UI for prefilling an edit form.
func RedactConfig(channelType string, raw json.RawMessage) json.RawMessage {
	m := decodeObject(raw)
	if m == nil {
		return json.RawMessage("{}")
	}
	for _, k := range secretConfigFields[channelType] {
		delete(m, k)
	}
	if channelType == "email" {
		if smtp, ok := m["smtp"].(map[string]any); ok {
			delete(smtp, "password")
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage("{}")
	}
	return out
}

// MergeSecrets reconciles the secret fields of submitted against stored, keyed
// on presence so the operator keeps control:
//
//   - secret ABSENT from submitted   → keep the stored value (unchanged edit)
//   - secret PRESENT but empty ("")  → clear it (explicit removal)
//   - secret PRESENT with a value    → use the re-entered value
//
// Non-secret fields are always taken from submitted verbatim.
func MergeSecrets(channelType string, stored, submitted json.RawMessage) json.RawMessage {
	sub := decodeObject(submitted)
	if sub == nil {
		return submitted
	}
	st := decodeObject(stored)

	for _, k := range secretConfigFields[channelType] {
		if _, present := sub[k]; !present && st != nil && !isBlankVal(st[k]) {
			sub[k] = st[k]
		}
	}
	if channelType == "email" {
		if subSMTP, ok := sub["smtp"].(map[string]any); ok && st != nil {
			if stSMTP, ok := st["smtp"].(map[string]any); ok {
				if _, present := subSMTP["password"]; !present && !isBlankVal(stSMTP["password"]) {
					subSMTP["password"] = stSMTP["password"]
				}
			}
		}
	}

	out, err := json.Marshal(sub)
	if err != nil {
		return submitted
	}
	return out
}

func decodeObject(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}

func isBlankVal(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == ""
}
