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
//	          - pkg: github.com/foo/bar
//	            type: MyStruct
//	            field: SomeField
package gclplugin

import (
	"fmt"
	"strings"

	"github.com/golangci/plugin-module-register/register"
	nostructfieldassign "github.com/tigrato/teleport/build.assets/tools/linters/nostructfieldassign"
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
func New(settings any) (register.LinterPlugin, error) {
	s, ok := settings.(map[string]any)
	if !ok && settings != nil {
		return nil, fmt.Errorf("nostructfieldassign: expected settings to be a map, got %T", settings)
	}

	var fields []string
	if s != nil {
		raw, ok := s["fields"]
		if ok {
			entries, ok := raw.([]interface{})
			if !ok {
				return nil, fmt.Errorf("nostructfieldassign: \"fields\" must be a list, got %T", raw)
			}
			for i, item := range entries {
				entry, ok := item.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("nostructfieldassign: fields[%d] must be a map with pkg/type/field keys, got %T", i, item)
				}
				pkg, _ := entry["pkg"].(string)
				typ, _ := entry["type"].(string)
				field, _ := entry["field"].(string)
				if pkg == "" || typ == "" || field == "" {
					return nil, fmt.Errorf("nostructfieldassign: fields[%d] must have non-empty pkg, type, and field", i)
				}
				fields = append(fields, pkg+"."+typ+"."+field)
			}
		}
	}

	return &Plugin{fields: fields}, nil
}

// Plugin is the nostructfieldassign plugin wrapper for golangci-lint.
type Plugin struct {
	fields []string
}

// BuildAnalyzers returns the nostructfieldassign analyzer with the configured fields applied.
func (p *Plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	if len(p.fields) > 0 {
		if err := nostructfieldassign.Analyzer.Flags.Set("fields", strings.Join(p.fields, ",")); err != nil {
			return nil, fmt.Errorf("nostructfieldassign: set fields flag: %w", err)
		}
	}
	return []*analysis.Analyzer{nostructfieldassign.Analyzer}, nil
}

// GetLoadMode returns the load mode required by nostructfieldassign (types info).
func (p *Plugin) GetLoadMode() string { return register.LoadModeTypesInfo }
