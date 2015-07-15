package cp

import (
	"html/template"
	"path/filepath"
)

var templates map[string]*template.Template

func (s *CPHandler) Path(params ...string) string {
	out := append([]string{"/", s.prefix}, params...)
	return filepath.Join(out...)
}

func (s *CPHandler) initTemplates(baseDir string) {
	tpls := []tpl{
		tpl{name: "login", include: []string{"assets/static/tpl/login.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "keys", include: []string{"assets/static/tpl/keys.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "events", include: []string{"assets/static/tpl/events.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "webtuns", include: []string{"assets/static/tpl/webtuns.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "servers", include: []string{"assets/static/tpl/servers.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "sessions", include: []string{"assets/static/tpl/sessions.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "session", include: []string{"assets/static/tpl/session.tpl", "assets/static/tpl/base.tpl"}},
	}
	templates = make(map[string]*template.Template)
	for _, t := range tpls {
		tpl := template.New(t.name)
		tpl.Funcs(map[string]interface{}{
			"Path": s.Path,
		})
		for _, i := range t.include {
			template.Must(tpl.ParseFiles(filepath.Join(baseDir, i)))
		}
		templates[t.name] = tpl
	}
}

type tpl struct {
	name    string
	include []string
}
