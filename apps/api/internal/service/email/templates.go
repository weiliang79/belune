package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	texttemplate "text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// templateDef holds parsed text and HTML templates for a given template ID.
type templateDef struct {
	subject  string
	textTmpl *texttemplate.Template
	htmlTmpl *template.Template
}

func loadTemplates() (map[string]*templateDef, error) {
	reg := make(map[string]*templateDef)

	entries := []struct {
		id      string
		subject string
	}{
		{"password_reset", "Reset your password"},
		{"user_invitation", "You've been invited to Belune"},
		{"alert_deploy_failed", "Deployment failed"},
		{"alert_build_failed", "Build failed"},
		{"alert_quota_threshold", "Quota threshold reached"},
	}

	for _, e := range entries {
		txtBytes, err := templateFS.ReadFile(fmt.Sprintf("templates/%s.txt.tmpl", e.id))
		if err != nil {
			return nil, fmt.Errorf("email: missing text template for %q: %w", e.id, err)
		}
		htmlBytes, err := templateFS.ReadFile(fmt.Sprintf("templates/%s.html.tmpl", e.id))
		if err != nil {
			return nil, fmt.Errorf("email: missing html template for %q: %w", e.id, err)
		}

		txtTmpl, err := texttemplate.New(e.id).Parse(string(txtBytes))
		if err != nil {
			return nil, fmt.Errorf("email: parse text template %q: %w", e.id, err)
		}
		htmlTmpl, err := template.New(e.id).Parse(string(htmlBytes))
		if err != nil {
			return nil, fmt.Errorf("email: parse html template %q: %w", e.id, err)
		}

		reg[e.id] = &templateDef{
			subject:  e.subject,
			textTmpl: txtTmpl,
			htmlTmpl: htmlTmpl,
		}
	}

	return reg, nil
}

// renderTemplate executes a registered template with the given data.
// Returns subject, text body, and HTML body.
func renderTemplate(reg map[string]*templateDef, id string, data any) (subject, textBody, htmlBody string, err error) {
	def, ok := reg[id]
	if !ok {
		return "", "", "", fmt.Errorf("email: unknown template %q", id)
	}

	var textBuf bytes.Buffer
	if err := def.textTmpl.Execute(&textBuf, data); err != nil {
		return "", "", "", fmt.Errorf("email: render text template %q: %w", id, err)
	}

	var htmlBuf bytes.Buffer
	if err := def.htmlTmpl.Execute(&htmlBuf, data); err != nil {
		return "", "", "", fmt.Errorf("email: render html template %q: %w", id, err)
	}

	return def.subject, textBuf.String(), htmlBuf.String(), nil
}
