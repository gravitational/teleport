// Package gclplugin implements the golangci-lint module plugin interface for
// nostructfieldassign to be used as a private linter in golangci-lint.
// See https://golangci-lint.run/plugins/module-plugins/.
//
// Example golangci-lint configuration:
//
//	linters:
//	  enable:
//	    - nostructfieldassign
//
//	linters-settings:
//	  custom:
//	    nostructfieldassign:
//	      type: module
//	      path: /path/to/custom-gcl
//	      description: Forbids direct assignment to configured struct fields.
//	      original-url: github.com/gravitational/teleport/build.assets/tools/linters/nostructfieldassign
//	      settings:
//	        fields:
//	          - pkg: github.com/aws/aws-sdk-go-v2/aws
//	            type: Config
//	            field: Region
//	            msg: use WithRegion() option instead
//	          - pkg: github.com/foo/bar
//	            type: MyStruct
//	            field: SomeField
package gclplugin

import (
	"fmt"

	"github.com/golangci/plugin-module-register/register"
	nostructfieldassign "github.com/gravitational/teleport/build.assets/tools/linters/nostructfieldassign"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("nostructfieldassign", New)
}

// New returns the golangci-lint plugin for nostructfieldassign.
//
// Each entry in the "fields" settings list must be a map with the keys:
//
//	pkg   — the full import path of the package (e.g. "github.com/aws/aws-sdk-go-v2/aws")
//	type  — the struct type name (e.g. "Config")
//	field — the field name to forbid (e.g. "Region")
//	msg   — optional message shown in the diagnostic (e.g. "use WithRegion() option instead")
func New(settings any) (register.LinterPlugin, error) {
	return newPlugin(settings)
}

func newPlugin(settings any) (*Plugin, error) {
	s, ok := settings.(map[string]any)
	if !ok && settings != nil {
		return nil, fmt.Errorf("nostructfieldassign: expected settings to be a map, got %T", settings)
	}

	if s == nil {
		return &Plugin{}, nil
	}

	raw, ok := s["fields"]
	if !ok {
		return &Plugin{}, nil
	}

	var plugin Plugin
	if err := mapstructure.Decode(raw, &plugin.rules); err != nil {
		return nil, fmt.Errorf("invalid fields format; %w", err)
	}

	return &plugin, nil
}

// Plugin is the nostructfieldassign plugin wrapper for golangci-lint.
type Plugin struct {
	rules []nostructfieldassign.Rule
}

// BuildAnalyzers returns the nostructfieldassign analyzer with the configured fields applied.
func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{nostructfieldassign.NewAnalyzer(p.rules...)}, nil
}

// GetLoadMode returns the load mode required by nostructfieldassign (types info).
func (p *Plugin) GetLoadMode() string { return register.LoadModeTypesInfo }
