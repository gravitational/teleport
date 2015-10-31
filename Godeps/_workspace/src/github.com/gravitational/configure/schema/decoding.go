package schema

import (
	"encoding/json"
	"strings"
)

type configV1 struct {
	Params []paramSpec `json:"params"`
}

type paramSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Type        string          `json:"type"`
	Check       string          `json:"check"`
	Default     string          `json:"default"`
	CLI         cliSpec         `json:"cli"` // cli-specific settings
	Env         string          `json:"env"` // environment variable name
	Required    bool            `json:"required"`
	S           json.RawMessage `json:"spec"`
}

func (p *paramSpec) common() paramCommon {
	return paramCommon{
		name:  p.Name,
		descr: p.Description,
		check: p.Check,
		def:   p.Default,
		cli:   p.CLI,
		req:   p.Required,
		env:   p.Env,
	}
}

type paramCommon struct {
	name  string
	descr string
	check string
	req   bool
	cli   cliSpec
	def   string
	env   string
}

func (p *paramCommon) EnvName() string {
	if p.env != "" {
		return p.env
	}
	return strings.ToUpper(p.name)
}

func (p *paramCommon) CLIName() string {
	if p.cli.Name != "" {
		return p.cli.Name
	}
	return p.name
}

func (p *paramCommon) Name() string {
	return p.name
}

func (p *paramCommon) Description() string {
	return p.descr
}

func (p *paramCommon) Check() string {
	return p.check
}

func (p *paramCommon) Required() bool {
	return p.req
}

func (p *paramCommon) Default() string {
	return p.def
}

type cliSpec struct {
	Name string `json:"name"`
	Type string `json:"type"` // type is either 'flag' or 'arg', 'flag is the default'
}

func (s *paramSpec) Spec() paramSpec {
	return *s
}

type kvSpec struct {
	Separator string      `json:"separator"`
	Keys      []paramSpec `json:"keys"`
}

type enumSpec struct {
	Values []string `json:"values"`
}
