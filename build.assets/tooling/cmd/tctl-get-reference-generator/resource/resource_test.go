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

package resource

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// replaceBackticks replaces the "BACKTICK" placeholder text with backticks so
// we can include struct tags within source fixtures.
func replaceBackticks(source string) string {
	return strings.ReplaceAll(source, "BACKTICK", "`")
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
