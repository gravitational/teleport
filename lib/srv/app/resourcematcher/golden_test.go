/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package resourcematcher

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// update regenerates the generated sections (app_resources_desugared and each
// case's expect) of every golden file in place. Run
// "go test ./lib/srv/app/resourcematcher/ -run TestGolden -update" after
// changing engine behavior or adding a case, then review the diff.
var update = flag.Bool("update", false, "regenerate golden testdata files")

// A golden case is one YAML file under testdata/. The whole file is one
// document so it loads natively in Go and in the web playground, with no custom
// parser. Its keys:
//
//	description:                optional human note, shown in the playground.
//	app_resources:              authored sugared rules (paths/methods/where),
//	                            with no surrounding role. Role selection and
//	                            role-holding are a separate, upstream concern, so
//	                            the engine golden tests carry only the rule set.
//	app_resources_expressions:  bare predicate strings, the parallel of
//	                            node_labels_expression. When app_resources is
//	                            present, this is generated: each app_resources
//	                            entry lowers to exactly one entry here, in order.
//	                            When app_resources is absent, it is authored.
//	identity:                   optional default identity, overridable per case.
//	cases:                      a list of {request, identity?, expect}.
//	error:                      optional, the rules are expected to fail to
//	                            compile.
//
// The runner asserts: the sugared rules evaluate to the stored expect, the bare
// app_resources_expressions reach the same decision, and, when app_resources is
// present, the stored app_resources_expressions equals the freshly generated
// one-for-one lowering. So one golden decision pins both authoring surfaces and
// the lowering between them. The deny code lowers to a deny_hint wrapper, so the
// bare rules carry the same deny hints as the sugared ones and the two are
// compared on the whole decision, hints included.
type goldenFile struct {
	Description             string      `yaml:"description,omitempty"`
	AppResources            []Rule      `yaml:"app_resources,omitempty"`
	AppResourcesExpressions []string    `yaml:"app_resources_expressions,omitempty"`
	Identity                *tcIdentity `yaml:"identity,omitempty"`
	Cases                   []tcCase    `yaml:"cases,omitempty"`
	Error                   string      `yaml:"error,omitempty"`
}

// tcIdentity is the caller identity in golden form, every field omitempty so a
// file carries only what it sets, with no noise like an empty traits map.
type tcIdentity struct {
	Name   string              `yaml:"name,omitempty"`
	Roles  []string            `yaml:"roles,omitempty"`
	Traits map[string][]string `yaml:"traits,omitempty"`
}

func (i *tcIdentity) toIdentity() Identity {
	if i == nil {
		return Identity{}
	}
	return Identity{Name: i.Name, Roles: i.Roles, Traits: i.Traits}
}

// tcCase is one request exercised against the file's rules.
type tcCase struct {
	Request  Request     `yaml:"request"`
	Identity *tcIdentity `yaml:"identity,omitempty"`
	Expect   tcExpect    `yaml:"expect"`
}

// tcExpect is the flattened, golden-friendly view of a Decision. It omits the
// evaluated roles, which are a role-selection concern rather than an engine
// decision and are uninteresting under the single synthetic role.
type tcExpect struct {
	Allowed     bool              `yaml:"allowed"`
	Vars        map[string]string `yaml:"vars,omitempty"`
	AllowCode   string            `yaml:"allow_code,omitempty"`
	AllowReason string            `yaml:"allow_reason,omitempty"`
	DenyKind    string            `yaml:"deny_kind,omitempty"`
	DenyHints   []Hint            `yaml:"deny_hints,omitempty"`
	Error       string            `yaml:"error,omitempty"`
}

func TestGolden(t *testing.T) {
	var files []string
	require.NoError(t, filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".yaml") {
			files = append(files, path)
		}
		return nil
	}))
	require.NotEmpty(t, files, "no golden cases under testdata/")

	for _, file := range files {
		name := strings.TrimSuffix(strings.TrimPrefix(file, "testdata/"), ".yaml")
		t.Run(name, func(t *testing.T) {
			runGolden(t, file)
		})
	}
}

func runGolden(t *testing.T, file string) {
	raw, err := os.ReadFile(file)
	require.NoError(t, err)
	var g goldenFile
	require.NoError(t, yaml.Unmarshal(raw, &g), "parsing %s", file)

	hasSugar := len(g.AppResources) > 0

	// A file may assert that its rules do not compile, on whichever surface it
	// authored.
	if g.Error != "" {
		_, err := CompileRoles([]Role{{Name: "test", Resources: g.AppResources, Expressions: g.AppResourcesExpressions}})
		require.Error(t, err)
		require.Contains(t, err.Error(), g.Error)
		return
	}

	// When app_resources is present, app_resources_expressions is its generated
	// lowering: each rule lowers to exactly one entry, in order. The sugared
	// surface is then evaluated alongside the bare one to prove the two decide a
	// request identically.
	var sugaredSet RoleSet
	var wantExpressions []string
	if hasSugar {
		sugaredRole := Role{Name: "test", Resources: g.AppResources}
		sugaredSet, err = CompileRoles([]Role{sugaredRole})
		require.NoError(t, err, "compiling sugared rules")
		wantExpressions, err = DesugarResources(sugaredRole)
		require.NoError(t, err, "desugaring resources")
		if len(wantExpressions) == 0 {
			wantExpressions = nil
		}
	}

	// The bare surface is app_resources_expressions. For a sugared file it is the
	// freshly generated lowering, not the stored block, so a stale stored block
	// is caught by the comparison below rather than failing to compile here; for
	// an authored-bare file it is what the file carries.
	bareExpressions := g.AppResourcesExpressions
	if hasSugar {
		bareExpressions = wantExpressions
	}
	bareSet, err := CompileRoles([]Role{{Name: "test", Expressions: bareExpressions}})
	require.NoError(t, err, "compiling app_resources_expressions")

	defaultIdentity := g.Identity.toIdentity()

	for i := range g.Cases {
		identity := defaultIdentity
		if g.Cases[i].Identity != nil {
			identity = g.Cases[i].Identity.toIdentity()
		}
		// The bare app_resources_expressions are the surface every file carries.
		// A sugared file additionally evaluates app_resources, and the two must
		// reach the same decision, proving the declarative form and its lowering
		// decide a request identically. The authoritative expect is the sugared
		// decision when sugar is present, since the deny code is a sugar-only
		// feature, and the bare decision otherwise.
		fromBare := evaluate(bareSet, g.Cases[i].Request, identity)
		if *update {
			if hasSugar {
				g.Cases[i].Expect = evaluate(sugaredSet, g.Cases[i].Request, identity)
			} else {
				g.Cases[i].Expect = fromBare
			}
			continue
		}
		at := fmt.Sprintf("case %d %s %s", i, g.Cases[i].Request.Method, g.Cases[i].Request.Path)
		if hasSugar {
			fromSugared := evaluate(sugaredSet, g.Cases[i].Request, identity)
			require.Equal(t, fromSugared, fromBare,
				"%s: sugared and app_resources_expressions reach different decisions", at)
			require.Equal(t, g.Cases[i].Expect, fromSugared,
				"%s: stored expect is stale; rerun with -update", at)
		} else {
			require.Equal(t, g.Cases[i].Expect, fromBare,
				"%s: stored expect is stale; rerun with -update", at)
		}
	}

	if hasSugar && !*update {
		require.Equal(t, wantExpressions, g.AppResourcesExpressions,
			"app_resources_expressions is stale; rerun with -update")
	}
	if !*update {
		return
	}

	// On -update, regenerate the app_resources_expressions block only when it is
	// the generated lowering of app_resources; an authored bare file keeps its
	// expressions untouched and only its expects are refreshed.
	require.NoError(t, rewriteGenerated(file, raw, wantExpressions, g.Cases))
}

// rewriteGenerated regenerates only the derived sections of a golden file, the
// generated app_resources_expressions block and each case's expect, while
// leaving every authored node, including its comments, byte-for-byte. It
// round-trips through a yaml.Node so a plain yaml.Marshal of the whole struct,
// which drops comments, is never used. The generated app_resources_expressions
// is placed right after app_resources, and each expect right after its request,
// so a freshly authored file gains them in a readable order.
func rewriteGenerated(file string, raw []byte, expressions []string, cases []tcCase) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if len(doc.Content) == 0 {
		return fmt.Errorf("%s: empty document", file)
	}
	root := doc.Content[0]
	// A file with no sugared app_resources has nothing to lower, so its
	// app_resources_expressions is authored and left untouched here.
	if len(expressions) > 0 {
		expressionsNode, err := encodeNode(expressions)
		if err != nil {
			return err
		}
		mapSetAfter(root, "app_resources", "app_resources_expressions", expressionsNode)
	}
	if casesNode := mapValue(root, "cases"); casesNode != nil {
		for i, caseNode := range casesNode.Content {
			if i >= len(cases) {
				break
			}
			expectNode, err := encodeNode(cases[i].Expect)
			if err != nil {
				return err
			}
			mapSetAfter(caseNode, "request", "expect", expectNode)
		}
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(file, out, 0o644)
}

// encodeNode renders a Go value into a fresh yaml.Node, the same content a
// yaml.Marshal would produce, so it can be spliced into a document tree.
func encodeNode(v any) (*yaml.Node, error) {
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		return nil, err
	}
	return &n, nil
}

// mapValue returns the value node for key in a mapping node, or nil.
func mapValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// mapSetAfter sets key to val in a mapping node. An existing key is replaced in
// place, preserving its position. A new key is inserted right after afterKey, or
// appended if afterKey is absent.
func mapSetAfter(m *yaml.Node, afterKey, key string, val *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = val
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == afterKey {
			rest := append([]*yaml.Node{}, m.Content[i+2:]...)
			m.Content = append(m.Content[:i+2], append([]*yaml.Node{keyNode, val}, rest...)...)
			return
		}
	}
	m.Content = append(m.Content, keyNode, val)
}

// evaluate runs a request and folds the Decision (or error) into the golden
// expect view.
func evaluate(set RoleSet, request Request, identity Identity) tcExpect {
	dec, err := set.Evaluate(request, identity)
	if err != nil {
		return tcExpect{Error: err.Error()}
	}
	e := tcExpect{Allowed: dec.Allowed}
	if dec.Allow != nil {
		e.Vars = nilIfEmptyMap(dec.Allow.Vars)
		e.AllowCode = dec.Allow.Code
		e.AllowReason = dec.Allow.Reason
	}
	if dec.Deny != nil {
		e.DenyKind = string(dec.Deny.Kind)
		if len(dec.Deny.Hints) > 0 {
			e.DenyHints = dec.Deny.Hints
		}
	}
	return e
}

func nilIfEmptyMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return m
}
