/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

const (
	testMethodPrefix    = "Test"
	testingPkgPath      = "testing"
	testifySuitePkgPath = "github.com/stretchr/testify/suite"
	runMethodSig        = ".Run"
)

// Method represents method name and body SHA
type Method struct {
	// Name represents test method name
	Name string

	// SHA1 is the method's body hash used for determining whether the method has changed.
	SHA1 string

	// RefName represents method name for -run flag
	RefName string
}

// pkgRefNames represent package references to testify.suite and testing
type pkgRefNames struct {
	// testing reference name for "testing" package
	testing string

	// testifySuite reference name for "github.com/stretchr/testify/suite" package
	testifySuite string
}

type RunnersMap = map[string]map[string]string

// findAllSuiteRunners finds all suite runners related to changed files
func findAllSuiteRunners(repoPath string, filename []string) (RunnersMap, error) {
	s := StringSet{}

	// Find all affected directories
	for _, f := range filename {
		dir := filepath.Join(repoPath, path.Dir(f))
		s[dir] = struct{}{}
	}

	allRunners := make(RunnersMap)

	// Find all test files in affected directoriees
	for dir := range s {
		matches, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Find all suite runners
		for _, m := range matches {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, m, nil, 0)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			runners := findSuiteRunners(node, fset)
			key := path.Dir(m)

			r := allRunners[key]
			if r == nil {
				r = make(map[string]string)
			}

			for k, v := range runners {
				r[k] = v
			}

			allRunners[key] = r
		}
	}

	return allRunners, nil
}

// parseMethodMap returns an array of methods from a file
func parseMethodMap(filename string, src any, allRunners RunnersMap) ([]Method, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	runners := make(map[string]string)

	if allRunners != nil {
		r := allRunners[path.Dir(filename)]
		if r != nil {
			runners = r
		}
	}

	r := make([]Method, 0)

	// Find functions beginning with "Test"
	ast.Inspect(node, func(n ast.Node) bool {
		ret, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Skip anonymous methods
		if ret.Name == nil {
			return true
		}

		methodName := ret.Name.String()

		// Skip methods which do not start with Test
		if !strings.HasPrefix(methodName, testMethodPrefix) {
			return true
		}

		refName := methodName

		receiver := cleanup(getMethodReceiver(fset, ret))
		if receiver != "" {
			parent, ok := runners[receiver]
			if !ok {
				return true
			}
			refName = parent + "/" + refName
		}

		r = append(r, Method{methodName, getMethodBodyHash(fset, ret), refName})

		return true
	})

	return r, nil
}

// findSuiteRunners finds testify/suite root functions
func findSuiteRunners(node *ast.File, fset *token.FileSet) map[string]string {
	pkgs := findTestPkgImports(node)
	runners := make(map[string]string)

	// Find suite runners
	ast.Inspect(node, func(n ast.Node) bool {
		ret, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		if ret.Name == nil {
			return true
		}

		// Check if a method has the reference to a testify/suite.Run()
		ast.Inspect(ret.Body, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Fn call signature
			var sigBuf bytes.Buffer
			printer.Fprint(&sigBuf, fset, c.Fun)

			// Is suite.Run()
			if sigBuf.String() != pkgs.testifySuite+runMethodSig {
				return true
			}

			if len(c.Args) < 2 {
				return true
			}

			// Get the suite name from the second argument
			var suiteNameBuf bytes.Buffer
			printer.Fprint(&suiteNameBuf, fset, c.Args[1])

			runners[cleanup(suiteNameBuf.String())] = ret.Name.String()

			return true
		})

		return true
	})

	return runners
}

// findTestPkgImports finds declarations of testing and suite packages
func findTestPkgImports(f *ast.File) pkgRefNames {
	p := pkgRefNames{}

	for _, i := range f.Imports {
		v := strings.Trim(i.Path.Value, `"`)

		if v == testingPkgPath {
			p.testing = getPackageRef(i)
		}

		if v == testifySuitePkgPath {
			p.testifySuite = getPackageRef(i)
		}
	}

	return p
}

// getPackageRef returns global name, which is used for package reference
func getPackageRef(i *ast.ImportSpec) string {
	if i.Name != nil {
		return i.Name.String()
	}

	parts := strings.Split(strings.Trim(i.Path.Value, `"`), "/")
	return parts[len(parts)-1]
}

// getMethodReceiver returns method receiver
func getMethodReceiver(fset *token.FileSet, m *ast.FuncDecl) string {
	// If a method has a receiver
	if m.Recv == nil || len(m.Recv.List) == 0 {
		return ""
	}

	var b bytes.Buffer
	printer.Fprint(&b, fset, m.Recv.List[0].Type)

	return cleanup(b.String())
}

// getMethodBodyHash returns SHA512 hexstring of method body
func getMethodBodyHash(fset *token.FileSet, ret *ast.FuncDecl) string {
	hasher := sha1.New()
	printer.Fprint(hasher, fset, ret)

	return hex.EncodeToString(hasher.Sum(nil))
}

// cleanup removes &, *, {} from a string
func cleanup(s string) string {
	v := strings.ReplaceAll(s, "&", "")
	v = strings.ReplaceAll(v, "{}", "")
	v = strings.ReplaceAll(v, "*", "")

	return v
}
