package systemd

import (
	_ "embed"
	"text/template"
)

var (
	//go:embed systemd.tmpl
	templateData string
	Template     = template.Must(template.New("").Parse(templateData))
)

type TemplateParams struct {
	UnitName             string
	User                 string
	Group                string
	AnonymousTelemetry   bool
	ConfigPath           string
	TBotPath             string
	DiagSocketForUpdater string
}
