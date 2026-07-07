// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"testing"

	"golang.org/x/tools/go/analysis"
)

// messages extracts the Message field from each Finding, so tests can
// compare on message content without asserting on Pos.
func messages(findings []Finding) []string {
	msgs := make([]string, len(findings))
	for i, f := range findings {
		msgs[i] = f.Message
	}
	return msgs
}

func TestCheckTargets(t *testing.T) {
	tests := []struct {
		name    string
		pkgPath string
		// srcs holds the contents of one or more synthetic files in the
		// same package, so tests can exercise cross-file resolution (e.g. a
		// constant declared in a separate constants.go-style file).
		srcs    []string
		targets []Target
		want    []Finding
	}{
		{
			// Confirms the analyzer fails closed: a real violation exists here, but with
			// no targets given, nothing should be checked or reported.
			name:    "empty targets against a real violation",
			pkgPath: "p",
			srcs: []string{`package p

import "github.com/gravitational/trace"

func onExample() error {
	return trace.BadParameter("something went wrong")
}
`},
			targets: nil,
			want:    nil,
		},
		{
			// Mirrors the real onRequestSearch shape (two BadParameter calls in one
			// function); confirms the walk collects every violation, not just the first.
			name:    "two violations in the same target function",
			pkgPath: "p",
			srcs: []string{`package p

import "github.com/gravitational/trace"

func onExample() error {
	if true {
		return trace.BadParameter("only one of --kind and --roles may be specified")
	}
	return trace.BadParameter("one of --kind and --roles is required")
}
`},
			targets: []Target{
				{Package: "p", Function: "onExample"},
			},
			want: []Finding{
				{Message: "trace.BadParameter call does not link to Teleport documentation"},
				{Message: "trace.BadParameter call does not link to Teleport documentation"},
			},
		},
		{
			// Regression guard for the false positive hit scanning tool/tsh manually: a
			// non-Teleport URL (git.go's real link to git-scm.com) must not be mistaken for a
			// Teleport docs link.
			name:    "message with non-teleport url still reports",
			pkgPath: "p",
			srcs: []string{`package p

import "github.com/gravitational/trace"

func onExample() error {
	return trace.BadParameter("could not locate the executable, install it by following the instructions at https://git-scm.com/book/en/v2/Getting-Started-Installing-Git")
}
`},
			targets: []Target{
				{Package: "p", Function: "onExample"},
			},
			want: []Finding{
				{Message: "trace.BadParameter call does not link to Teleport documentation"},
			},
		},
		{
			name:    "bad parameter without docs url",
			pkgPath: "p",
			srcs: []string{`package p

import "github.com/gravitational/trace"

func onExample() error {
	return trace.BadParameter("something went wrong")
}
`},
			targets: []Target{
				{Package: "p", Function: "onExample"},
			},
			want: []Finding{
				{Message: "trace.BadParameter call does not link to Teleport documentation"},
			},
		},
		{
			name:    "bad parameter with docs url",
			pkgPath: "p",
			srcs: []string{`package p

import "github.com/gravitational/trace"

func onExample() error {
	return trace.BadParameter("something went wrong, see https://goteleport.com/docs/reference/example/ for details")
}
`},
			targets: []Target{
				{Package: "p", Function: "onExample"},
			},
			want: nil,
		},
		{
			// Currently fails: stringLiteralValue only resolves *ast.BasicLit, so a message
			// passed via a named constant is treated as "can't verify" and silently skipped.
			// This should report a finding once constant resolution is implemented.
			name:    "bad parameter with named constant missing docs url",
			pkgPath: "p",
			srcs: []string{`package p

import "github.com/gravitational/trace"

const noLinkMessage = "something went wrong"

func onExample() error {
	return trace.BadParameter(noLinkMessage)
}
`},
			targets: []Target{
				{Package: "p", Function: "onExample"},
			},
			want: []Finding{
				{Message: "trace.BadParameter call does not link to Teleport documentation"},
			},
		},
		{
			// Mirrors Teleport's constants.go convention: the constant lives in one file
			// (srcs[0]) and is referenced from another (srcs[1]). Exercises both named-constant
			// resolution and the multi-file lookup added to this harness in one case.
			name:    "bad parameter with named constant with docs url in a separate file",
			pkgPath: "p",
			srcs: []string{
				`package p

const helpfulMessage = "something went wrong, see https://goteleport.com/docs/reference/example/ for details"
`,
				`package p

import "github.com/gravitational/trace"

func onExample() error {
	return trace.BadParameter(helpfulMessage)
}
`,
			},
			targets: []Target{
				{Package: "p", Function: "onExample"},
			},
			want: nil,
		},
		{
			// Currently fails: a constant's value built from string concatenation is an
			// *ast.BinaryExpr, and neither stringLiteralValue nor resolveMessage's constant
			// lookup handles anything but a single literal. Mirrors tsh.go's
			// mlockFailureMessage, which is built the same way via `+`.
			name:    "bad parameter with concatenated constant missing docs url",
			pkgPath: "p",
			srcs: []string{`package p

import "github.com/gravitational/trace"

const noLinkMessage = "something went wrong. " +
	"please check your configuration."

func onExample() error {
	return trace.BadParameter(noLinkMessage)
}
`},
			targets: []Target{
				{Package: "p", Function: "onExample"},
			},
			want: []Finding{
				{Message: "trace.BadParameter call does not link to Teleport documentation"},
			},
		},
		{
			name:    "violation outside target function",
			pkgPath: "p",
			srcs: []string{`package p

import "github.com/gravitational/trace"

func onExample() error {
	return nil
}

func notATarget() error {
	return trace.BadParameter("something went wrong")
}
`},
			targets: []Target{
				{Package: "p", Function: "onExample"},
			},
			want: nil,
		},
		{
			// Mirrors a real collision found in tool/tsh, tool/tctl, and tool/tbot: each has
			// its own top-level Run function, and tool/tsh/common and tool/tctl/common even
			// share the identical bare package name "common". A target for one package's Run
			// must not pick up a same-named Run in a different package.
			name:    "different package, same function name",
			pkgPath: "github.com/gravitational/teleport/tool/tsh/common",
			srcs: []string{`package common

import "github.com/gravitational/trace"

func Run() error {
	return trace.BadParameter("something went wrong")
}
`},
			targets: []Target{
				{Package: "github.com/gravitational/teleport/tool/tctl/common", Function: "Run"},
			},
			want: nil,
		},
		{
			name:    "same function name, different receiver",
			pkgPath: "p",
			srcs: []string{`package p

import "github.com/gravitational/trace"

type TokenSpec struct{}

func (t *TokenSpec) CheckAndSetDefaults() error {
	return trace.BadParameter("something went wrong")
}

type AppSpec struct{}

func (a *AppSpec) CheckAndSetDefaults() error {
	return trace.BadParameter("something else went wrong")
}
`},
			targets: []Target{
				{Package: "p", Function: "CheckAndSetDefaults", Receiver: "TokenSpec"},
			},
			want: []Finding{
				{Message: "trace.BadParameter call does not link to Teleport documentation"},
			},
		},
	}

	// Every case in this table only ever exercises trace.BadParameter, so a
	// single shared constructor list covers all of them.
	constructors := []Target{
		{Package: "github.com/gravitational/trace", Function: "BadParameter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			var files []*ast.File
			for i, src := range tt.srcs {
				f, err := parser.ParseFile(fset, fmt.Sprintf("file%d.go", i), src, 0)
				if err != nil {
					t.Fatalf("parsing source %d: %v", i, err)
				}
				files = append(files, f)
			}

			pass := &analysis.Pass{
				Fset:  fset,
				Files: files,
				Pkg:   types.NewPackage(tt.pkgPath, files[0].Name.Name),
			}

			got := checkTargets(pass, tt.targets, constructors)

			if !slices.Equal(messages(got), messages(tt.want)) {
				t.Errorf("checkTargets() = %v, want %v", messages(got), messages(tt.want))
			}
		})
	}
}
