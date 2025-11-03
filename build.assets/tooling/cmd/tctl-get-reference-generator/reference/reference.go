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

package reference

import (
	_ "embed"
	"fmt"
	"go/ast"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/tctl-get-reference-generator/resource"
)

// TypeInfo represents the name and package name of an exported Go type. It
// makes no guarantees about whether the type was actually declared within the
// package.
type TypeInfo struct {
	// Go package path (not a file path)
	Package string `yaml:"package"`
	// Name of the type, e.g., Metadata
	Name string `yaml:"name"`
}

// getPackageInfoFromExpr extracts a package name and declaration name from an
// arbitrary expression. If the expression is not an expected kind,
// getPackageInfoFromExpr returns an empty PackageInfo.
func getPackageInfoFromExpr(expr ast.Expr) resource.PackageInfo {
	var gopkg, fldname string
	switch t := expr.(type) {
	case *ast.StarExpr:
		return getPackageInfoFromExpr(t.X)
	case *ast.SelectorExpr:
		// If the type of the field is an *ast.SelectorExpr,
		// it's of the form <package>.<type name>.
		g, ok := t.X.(*ast.Ident)
		if ok {
			gopkg = g.Name
		}
		fldname = t.Sel.Name

	// There's no package, so only assign a name.
	case *ast.Ident:
		fldname = t.Name
	}
	return resource.PackageInfo{
		DeclName:    fldname,
		PackageName: gopkg,
	}
}

// Generate uses the provided user-facing configuration to write the resource
// reference to fs.
func Generate() error {
	// TODO: have this select the correct path.
	// TODO: use the resulting sourceData
	_, err := resource.NewSourceData(".")
	if err != nil {
		return fmt.Errorf("loading Go source files: %w", err)
	}

	return nil
}
