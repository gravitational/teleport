package cp

import (
	"html/template"
)

var templates map[string]*template.Template

func init() {
	tpls := []tpl{
		tpl{"login", string(MustAsset("assets/static/tpl/login.tpl")), []string{"assets/static/tpl/base.tpl"}},
		tpl{"keys", string(MustAsset("assets/static/tpl/keys.tpl")), []string{"assets/static/tpl/base.tpl"}},
		tpl{"events", string(MustAsset("assets/static/tpl/events.tpl")), []string{"assets/static/tpl/base.tpl"}},
		tpl{"webtuns", string(MustAsset("assets/static/tpl/webtuns.tpl")), []string{"assets/static/tpl/base.tpl"}},
		tpl{"servers", string(MustAsset("assets/static/tpl/servers.tpl")), []string{"assets/static/tpl/base.tpl"}},
	}
	templates = make(map[string]*template.Template)
	for _, t := range tpls {
		tpl := template.New(t.name)
		template.Must(tpl.Parse(t.val))
		if len(t.include) == 0 {
			return
		}
		for _, i := range t.include {
			template.Must(tpl.Parse(string(MustAsset(i))))
		}
		templates[t.name] = tpl
	}
}

type tpl struct {
	name    string
	val     string
	include []string
}
