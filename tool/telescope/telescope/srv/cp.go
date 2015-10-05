package srv

import (
	"html/template"
	"path/filepath"
)

var templates map[string]*template.Template

func initTemplates(baseDir string) {
	tpls := []tpl{
		tpl{name: "login", include: []string{"assets/static/tpl/login.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "sites", include: []string{"assets/static/tpl/sites.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "site-servers", include: []string{"assets/static/tpl/site/servers.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "site-events", include: []string{"assets/static/tpl/site/events.tpl", "assets/static/tpl/base.tpl"}},
	}
	templates = make(map[string]*template.Template)
	for _, t := range tpls {
		tpl := template.New(t.name)
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
