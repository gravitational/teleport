// Teleport
// Copyright (C) 2025  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// replaceBackticks replaces the "BACKTICK" placeholder text with backticks so
// we can include struct tags within source fixtures.
func replaceBackticks(source string) string {
	return strings.ReplaceAll(source, "BACKTICK", "`")
}

func TestGetCollectionTypeCases(t *testing.T) {
	cases := []struct {
		description string
		source      string
		expected    []PackageInfo
	}{
		{
			description: "switch statement after other blocks",
			source: `package mypkg

func (rc *ResourceCommand) getCollection(ctx context.Context, client *authclient.Client) (resources.Collection, error) {
	if rc.ref.Kind == "" {
		return nil, trace.BadParameter("specify resource to list, e.g. 'tctl get roles'")
	}

	// Looking if the resource has been converted to the handler format.
	if coll, found := resources.Handlers()[rc.ref.Kind]; found {
		return coll, nil
	}
	// The resource hasn't been migrated yet, falling back to the old logic.

	switch rc.ref.Kind {
	case types.KindSAMLConnector:
		connectors, err := getSAMLConnectors(ctx, client, rc.ref.Name, rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &samlCollection{connectors}, nil
	case types.KindOIDCConnector:
		connectors, err := getOIDCConnectors(ctx, client, rc.ref.Name, rc.withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &oidcCollection{connectors}, nil
	}

	return nil, trace.BadParameter("getting %q is not supported", rc.ref.String())
}
`,
			expected: []PackageInfo{
				{
					PackageName: "types",
					DeclName:    "KindSAMLConnector",
				},
				{
					PackageName: "types",
					DeclName:    "KindOIDCConnector",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			decls, _, err := parseDeclsFromFile(strings.NewReader(replaceBackticks(c.source)), ".", "myfile.go")
			require.NoError(t, err)
			actual, err := getCollectionTypeCases(decls, "getCollection")
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}
func TestExtractHandlersKeys(t *testing.T) {
	cases := []struct {
		description string
		source      string
		expected    []PackageInfo
	}{
		{
			description: "three handlers",
			source: `package mypkg

// Handlers returns a map of Handler per kind.
// This map will be filled as we convert existing resources
// to the Handler format.
func Handlers() map[string]Handler {
	// When adding resources, please keep the map alphabetically ordered.
	return map[string]Handler{
		types.KindAccessGraphSettings:                accessGraphSettingsHandler(),
		types.KindApp:                                appHandler(),
		types.KindAppServer:                          appServerHandler(),
	}
}`,
			expected: []PackageInfo{
				{
					PackageName: "types",
					DeclName:    "KindAccessGraphSettings",
				},
				{
					PackageName: "types",
					DeclName:    "KindApp",
				},
				{
					PackageName: "types",
					DeclName:    "KindAppServer",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			decls, _, err := parseDeclsFromFile(strings.NewReader(replaceBackticks(c.source)), ".", "myfile.go")
			require.NoError(t, err)
			actual, err := extractHandlersKeys(decls, "Handlers")
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}

func TestNamedImports(t *testing.T) {
	cases := []struct {
		description string
		input       string
		expected    map[string]string
	}{
		{
			description: "single-line format",
			input: `package mypkg
import alias "otherpkg"
`,
			expected: map[string]string{
				"alias": "otherpkg",
			},
		},
		{
			description: "multi-line format",
			input: `package mypkg
import (
    alias "first"
    alias2 "second"
)
`,
			expected: map[string]string{
				"alias":  "first",
				"alias2": "second",
			},
		},
		{
			description: "multi-segment package path",
			input: `package mypkg
import alias "my/multi/segment/package"
`,
			expected: map[string]string{
				"alias": "package",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset,
				"myfile.go",
				c.input,
				parser.ParseComments,
			)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, NamedImports(f))
		})
	}
}

func TestGetTopLevelStringAssignments(t *testing.T) {
	cases := []struct {
		description string
		source      string
		expected    map[PackageInfo]string
	}{
		{

			description: "single-var assignments",
			source: `package mypkg
var myString string = "This is a string"
var otherString string ="This is another string"
`,
			expected: map[PackageInfo]string{
				PackageInfo{
					DeclName:    "myString",
					PackageName: "mypkg",
				}: "This is a string",
				PackageInfo{
					DeclName:    "otherString",
					PackageName: "mypkg",
				}: "This is another string",
			},
		},
		{

			description: "single-const assignments",
			source: `package mypkg
const myString string = "This is a string"
const otherString string ="This is another string"
`,
			expected: map[PackageInfo]string{
				PackageInfo{
					DeclName:    "myString",
					PackageName: "mypkg",
				}: "This is a string",
				PackageInfo{
					DeclName:    "otherString",
					PackageName: "mypkg",
				}: "This is another string",
			},
		},
		{

			description: "multiple-var assignments",
			source: `package mypkg

var (
  myString string = "This is a string"
  otherString string ="This is another string"
)
`,
			expected: map[PackageInfo]string{
				PackageInfo{
					DeclName:    "myString",
					PackageName: "mypkg",
				}: "This is a string",
				PackageInfo{
					DeclName:    "otherString",
					PackageName: "mypkg",
				}: "This is another string",
			},
		},
		{

			description: "multiple-const assignments",
			source: `package mypkg

const (
  myString string = "This is a string"
  otherString string ="This is another string"
)
`,
			expected: map[PackageInfo]string{
				PackageInfo{
					DeclName:    "myString",
					PackageName: "mypkg",
				}: "This is a string",
				PackageInfo{
					DeclName:    "otherString",
					PackageName: "mypkg",
				}: "This is another string",
			},
		},
		{

			description: "mix of string and non-string vars and consts",
			source: `package mypkg

import "strings"

const (
  stringConst string = "This is a string"
  boolConst  bool = false
)

var (
    stringVar string = "This is a string"
    funcConst string = strings.ToLower("HELLO") 
)
`,

			expected: map[PackageInfo]string{
				PackageInfo{
					DeclName:    "stringVar",
					PackageName: "mypkg",
				}: "This is a string",
				PackageInfo{
					DeclName:    "stringConst",
					PackageName: "mypkg",
				}: "This is a string",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			fset := token.NewFileSet()
			d, err := parser.ParseFile(fset,
				"myfile.go",
				replaceBackticks(c.source),
				parser.ParseComments,
			)
			if err != nil {
				t.Fatalf("test fixture contains invalid Go source: %v\n", err)
			}

			actual, err := GetTopLevelStringAssignments(d.Decls, d.Name.Name)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}
