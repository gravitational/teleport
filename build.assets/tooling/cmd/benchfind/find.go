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
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/tools/go/packages"
)

// Config specifies configuration for finding benchmark packages.
type Config struct {
	// Patterns specifies package patterns to search. Defaults to './...' if empty.
	Patterns []string
	// BuildTags specifies build tags to use when loading packages.
	BuildTags string
	// Dir specifies the working directory to run package discovery from.
	Dir string
	// Excludes specifies package path prefixes to exclude.
	Excludes []string
}

// Loader abstracts package loading.
type Loader interface {
	Load(cfg *packages.Config, patterns ...string) ([]*packages.Package, error)
}

// PackagesLoader is the default Loader using go/packages.
type PackagesLoader struct{}

func (PackagesLoader) Load(cfg *packages.Config, patterns ...string) ([]*packages.Package, error) {
	return packages.Load(cfg, patterns...)
}

// Find finds all packages matching the given patterns that contain benchmarks.
func Find(cfg Config) ([]string, error) {
	return findWithLoader(cfg, PackagesLoader{})
}

func findWithLoader(cfg Config, loader Loader) ([]string, error) {
	if len(cfg.Patterns) == 0 {
		cfg.Patterns = []string{"./..."}
	}

	pkgs, err := loader.Load(&packages.Config{
		Dir: cfg.Dir,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax, // requires [packages.NeedFiles] to be set.
		Tests:      true,
		BuildFlags: buildFlags(cfg.BuildTags),
	}, cfg.Patterns...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if packages.PrintErrors(pkgs) > 0 {
		return nil, trace.BadParameter("failed to load packages")
	}

	return findBenchmarks(pkgs, cfg.Excludes), nil
}

func findBenchmarks(pkgs []*packages.Package, excludes []string) []string {
	seen := make(map[string]struct{})
	var result []string

	packages.Visit(pkgs, nil, func(p *packages.Package) {
		path := strings.TrimSuffix(p.PkgPath, ".test")
		path = strings.TrimSuffix(path, "_test")

		if matchesAnyPrefix(path, excludes) {
			return
		}

		if pkgsContainBenchmarks(p) {
			if _, ok := seen[path]; !ok {
				seen[path] = struct{}{}
				result = append(result, path)
			}
		}
	})

	return result
}

func buildFlags(tags string) []string {
	if tags == "" {
		return nil
	}
	return []string{"-tags=" + tags}
}

func pkgsContainBenchmarks(pkg *packages.Package) bool {
	return slices.ContainsFunc(pkg.Syntax, filepkgsContainBenchmarks)
}

func filepkgsContainBenchmarks(file *ast.File) bool {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if isBenchmark(fn) {
			return true
		}
	}
	return false
}

func matchesAnyPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func isBenchmark(fn *ast.FuncDecl) bool {
	if fn.Recv != nil {
		return false
	}
	if fn.Name == nil || !strings.HasPrefix(fn.Name.Name, "Benchmark") {
		return false
	}
	if fn.Type == nil || fn.Type.Params == nil {
		return false
	}

	params := fn.Type.Params.List
	if len(params) != 1 {
		return false
	}

	star, ok := params[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	return sel.Sel.Name == "B"
}
