package report

import (
	"html/template"
	"io"
	"strings"

	"github.com/gravitational/teleport/lib/tmig/classify"
)

// htmlFuncMap provides helpers available inside the HTML template.
var htmlFuncMap = template.FuncMap{
	"verdictClass": func(v classify.Verdict) string {
		switch v {
		case classify.VerdictAuto:
			return "auto"
		case classify.VerdictPrereq:
			return "prereq"
		case classify.VerdictPipeline:
			return "pipeline"
		case classify.VerdictManual:
			return "manual"
		default:
			return "orphan"
		}
	},
	"statusText": func(s classify.Status) string {
		return strings.ToLower(string(s))
	},
	"attentionText": func(a classify.AttentionClass) string {
		switch a {
		case classify.AttentionNone:
			return "none"
		case classify.AttentionIaCOnetime:
			return "IaC"
		case classify.AttentionPipeline:
			return "pipeline"
		case classify.AttentionManual:
			return "per-host"
		default:
			return string(a)
		}
	},
	"roleOK": func(roles map[string]bool, name string) bool {
		return roles[name]
	},
	"verdictCount": func(m map[classify.Verdict]int, key string) int {
		return m[classify.Verdict(key)]
	},
}

var htmlTmpl = template.Must(template.New("report").Funcs(htmlFuncMap).Parse(htmlTemplateSource))

// RenderHTML writes a self-contained HTML readiness report.
func RenderHTML(rpt *Report, w io.Writer) error {
	return htmlTmpl.Execute(w, rpt)
}
