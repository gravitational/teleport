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
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

type fakeLoader struct {
	pkgs []*packages.Package
	err  error
}

func (f fakeLoader) Load(_ *packages.Config, _ ...string) ([]*packages.Package, error) {
	return f.pkgs, f.err
}

func mustParseFile(t *testing.T, src string) *ast.File {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)
	return file
}

func TestFindWithLoader(t *testing.T) {
	benchFile := mustParseFile(t, `
		package p
		import "testing"
		func BenchmarkFoo(b *testing.B) {}
	`)

	nonBenchFile := mustParseFile(t, `
		package p
		func Foo() {}
	`)

	loader := fakeLoader{
		pkgs: []*packages.Package{
			{
				PkgPath: "github.com/foo/bar",
				Syntax:  []*ast.File{benchFile},
			},
			{
				PkgPath: "github.com/baz/bar",
				Syntax:  []*ast.File{nonBenchFile},
			},
		},
	}

	cfg := Config{
		Patterns: []string{"./..."},
	}

	result, err := findWithLoader(cfg, loader)
	require.NoError(t, err)
	require.Equal(t, []string{"github.com/foo/bar"}, result)
}

func TestFindWithLoader_Excludes(t *testing.T) {
	benchFile := mustParseFile(t, `
		package p
		import "testing"
		func BenchmarkFoo(b *testing.B) {}
	`)

	loader := fakeLoader{
		pkgs: []*packages.Package{
			{
				PkgPath: "github.com/foo/bar",
				Syntax:  []*ast.File{benchFile},
			},
		},
	}

	cfg := Config{
		Excludes: []string{"github.com/foo"},
	}

	result, err := findWithLoader(cfg, loader)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestFindBenchmarks_DeduplicatesTestPackages(t *testing.T) {
	benchFile := mustParseFile(t, `
		package p
		import "testing"
		func BenchmarkFoo(b *testing.B) {}
	`)

	pkgs := []*packages.Package{
		{
			PkgPath: "github.com/foo/bar",
			Syntax:  []*ast.File{benchFile},
		},
		{
			PkgPath: "github.com/foo/bar.test",
			Syntax:  []*ast.File{benchFile},
		},
	}

	result := findBenchmarks(pkgs, nil)
	require.Equal(t, []string{"github.com/foo/bar"}, result)
}

func TestIsBenchmark(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "valid",
			src: `
				package p
				import "testing"
				func BenchmarkFoo(b *testing.B) {}
			`,
			want: true,
		},
		{
			name: "wrong name",
			src: `
				package p
				import "testing"
				func TestFoo(b *testing.B) {}
			`,
			want: false,
		},
		{
			name: "wrong param type",
			src: `
				package p
				func BenchmarkFoo(i int) {}
			`,
			want: false,
		},
		{
			name: "too many params",
			src: `
				package p
				import "testing"
				func BenchmarkFoo(b *testing.B, i int) {}
			`,
			want: false,
		},
		{
			name: "missing params",
			src: `
				package p
				func Foo() {}
			`,
			want: false,
		},
		{
			name: "cannot be a method",
			src: `
				package p
				import "testing"
				type T struct{}
				func (T) BenchmarkFoo(b *testing.B) {}
			`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := mustParseFile(t, tt.src)

			var fn *ast.FuncDecl
			for _, d := range file.Decls {
				if f, ok := d.(*ast.FuncDecl); ok {
					fn = f
					break
				}
			}
			assert.NotNil(t, fn)
			assert.Equal(t, tt.want, isBenchmark(fn))

			pkg := &packages.Package{
				Syntax: []*ast.File{file},
			}
			assert.Equal(t, tt.want, pkgsContainBenchmarks(pkg))
		})
	}
}

func TestMatchesAnyPrefix(t *testing.T) {
	require.False(t, matchesAnyPrefix("github.com/foo/bar", nil))
	require.True(t, matchesAnyPrefix("github.com/foo/bar", []string{"github.com/foo"}))
	require.False(t, matchesAnyPrefix("github.com/foo/bar", []string{"github.com/other"}))
}

func TestBuildFlags(t *testing.T) {
	require.Nil(t, buildFlags(""))
	require.Equal(t, []string{"-tags=foo"}, buildFlags("foo"))
	require.Equal(t, []string{"-tags=foo,bar"}, buildFlags("foo,bar"))
}

// TestFind is a smoketest that ensures Find works end-to-end.
func TestFind(t *testing.T) {
	dir := filepath.Join(".", "testdata")
	expected := []string{
		"github.com/gravitational/teleport/build.assets/tooling/cmd/benchfind/testdata",
		"github.com/gravitational/teleport/build.assets/tooling/cmd/benchfind/testdata/nested",
	}

	pkgs, err := Find(Config{
		Dir: dir,
	})
	require.NoError(t, err)
	require.Equal(t, expected, pkgs)
}
