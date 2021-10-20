/*
Copyright 2021 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateGoPkgs(t *testing.T) {
	goFile := `package main

import "mod/path"
import "other/mod/path"
import (
	"mod/path"
	alias "mod/path"

	"other/mod/path"
)
`

	// Write go file to memory so we can run it through the update function
	// Note: We use os.MkdirTemp here because "golang.org/x/tools/go/packages.Load"
	// is unable to read from t.TempDir()
	pkgDir, err := os.MkdirTemp("./", "pkg")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(pkgDir)) })

	goFilePath := filepath.Join(pkgDir, "main.go")
	err = os.WriteFile(goFilePath, []byte(goFile), fs.ModePerm)
	require.NoError(t, err)

	// run go file through update function
	err = updateGoPkgs(pkgDir, "mod/path", "updated/mod/path", nil)
	require.NoError(t, err)

	// read go file
	updatedGoFile, err := os.ReadFile(goFilePath)
	require.NoError(t, err)

	// compare result with expected result
	expectedGoFile := strings.ReplaceAll(goFile, "\"mod/path\"", "\"updated/mod/path\"")
	require.Equal(t, expectedGoFile, string(updatedGoFile))
}

func TestUpdateProtoFiles(t *testing.T) {
	protoFile := `syntax = "proto3";
package proto;

import "mod/path/types.proto";

message Example {
	types.Type field = 6 [
        (gogoproto.customtype) = "mod/path/types.Traits"
    ];
}
`

	// Write proto file to memory
	rootDir := t.TempDir()
	protoFilePath := filepath.Join(rootDir, "proto.proto")
	err := os.WriteFile(protoFilePath, []byte(protoFile), fs.ModePerm)
	require.NoError(t, err)

	// run proto file through update function
	err = updateProtoFiles(rootDir, "mod/path", "updated/mod/path")
	require.NoError(t, err)

	// read proto file
	updatedProtoFile, err := os.ReadFile(protoFilePath)
	require.NoError(t, err)

	// compare result with expected result
	expectedProtoFile := strings.ReplaceAll(protoFile, "mod/path", "updated/mod/path")
	require.Equal(t, expectedProtoFile, string(updatedProtoFile))
}

func TestUpdateGoModulePath(t *testing.T) {
	testUpdate := func(oldModPath, newModPath, newModVersion, oldModFile, expectedNewModFile string) func(t *testing.T) {
		return func(t *testing.T) {
			// Write mod file to memory so we can run it through the update function
			modDir := t.TempDir()
			modFilePath := filepath.Join(modDir, "go.mod")
			err := os.WriteFile(modFilePath, []byte(oldModFile), fs.ModePerm)
			require.NoError(t, err)

			err = updateGoModFile(modDir, oldModPath, newModPath, newModVersion)
			require.NoError(t, err)

			// Read the updated mod file from memory and compare it to the expected mod file
			updatedModFile, err := os.ReadFile(modFilePath)
			require.NoError(t, err)
			require.Equal(t, expectedNewModFile, string(updatedModFile))
		}
	}

	t.Run("updated module in header", testUpdate("go/mod/header", "updated/go/mod/header", "1.2.3",
		newGoModFileString("go/mod/header"),
		newGoModFileString("updated/go/mod/header"),
	))

	t.Run("updated module in statements", testUpdate("mod/path", "updated/mod/path", "1.2.3",
		newGoModFileString("go/mod/header", requireStatement("mod/path", "0.1.2")),
		newGoModFileString("go/mod/header", requireStatement("updated/mod/path", "1.2.3")),
	))

	t.Run("updated module not in mod file", testUpdate("mod/path", "updated/mod/path", "1.2.3",
		newGoModFileString("go/mod/header", requireStatement("other/mod/path", "0.1.2")),
		newGoModFileString("go/mod/header", requireStatement("other/mod/path", "0.1.2")),
	))

	// Create a go mod file with every type of go mod statement and
	// test that every statement and the header gets updated properly.
	testUpdateAllStatements := func(oldModPath, oldVersion, newVersion string) func(*testing.T) {
		oldModFile := newGoModFileString(oldModPath, allGoModStatements(oldModPath, oldVersion)...)
		newModPath := getNewModImportPath(oldModPath, newVersion)
		newModFile := newGoModFileString(newModPath, allGoModStatements(newModPath, newVersion)...)
		return testUpdate(oldModPath, newModPath, newVersion, oldModFile, newModFile)
	}

	t.Run("v0 to v0", testUpdateAllStatements("mod/path", "0.0.0", "0.0.0"))
	t.Run("v0 to v1", testUpdateAllStatements("mod/path", "0.0.0", "1.2.3"))
	t.Run("v0 to v1", testUpdateAllStatements("mod/path", "0.0.0", "1.2.3"))
	t.Run("v0 to v2", testUpdateAllStatements("mod/path", "0.0.0", "2.3.4"))
	t.Run("v2 to v3", testUpdateAllStatements("mod/path/v2", "2.3.4", "3.4.5"))
	t.Run("v1 to v0", testUpdateAllStatements("mod/path", "1.2.3", "0.0.0"))
	t.Run("v2 to v0", testUpdateAllStatements("mod/path/v2", "2.3.4", "0.0.0"))
}

// Create a go mod file as a string for testing
func newGoModFileString(modPath string, goModStatements ...string) string {
	header := fmt.Sprintf("module %v\n\ngo 1.15", modPath)
	return strings.Join(append([]string{header}, goModStatements...), "\n\n") + "\n"
}

// Helper functions for creating go mod statements
func requireStatement(path, version string) string {
	return fmt.Sprintf("require %v v%v", path, version)
}
func requireIndirect(path, version string) string {
	return fmt.Sprintf("require %v v%v // indirect", path, version)
}
func requireBlock(requires []string) string {
	for i, s := range requires {
		requires[i] = strings.ReplaceAll(s, "require ", "\t")
	}
	return fmt.Sprintf("require (\n%v\n)", strings.Join(requires, "\n"))
}
func replaceStatement(path string) string {
	return fmt.Sprintf("replace %v => path v1.2.3", path)
}
func replaceVersion(path, version string) string {
	return fmt.Sprintf("replace %v v%v => path v1.2.3", path, version)
}
func replaceBlock(replaces []string) string {
	for i, s := range replaces {
		replaces[i] = strings.ReplaceAll(s, "replace ", "\t")
	}
	return fmt.Sprintf("replace (\n%v\n)", strings.Join(replaces, "\n"))
}
func allGoModStatements(path, version string) []string {
	return []string{
		requireStatement(path, version),
		requireIndirect(path, version),
		requireBlock([]string{
			requireStatement(path, version),
			requireIndirect(path, version),
		}),
		replaceStatement(path),
		replaceVersion(path, version),
		replaceBlock([]string{
			replaceStatement(path),
			replaceVersion(path, version),
		}),
	}
}

func TestGetImportPaths(t *testing.T) {
	testGetImportPaths := func(currentModPath, newVersion, expectedNewModPath string) func(t *testing.T) {
		return func(t *testing.T) {
			// Write mod file to memory
			modDir := t.TempDir()
			modFilePath := filepath.Join(modDir, "go.mod")
			modFile := newGoModFileString(currentModPath)
			err := os.WriteFile(modFilePath, []byte(modFile), fs.ModePerm)
			require.NoError(t, err)

			// Get import paths using the mod file in memory and compare to expected results
			oldModPath, err := getModImportPath(modDir)
			require.NoError(t, err)
			require.Equal(t, currentModPath, oldModPath)
			newModPath := getNewModImportPath(oldModPath, newVersion)
			require.Equal(t, expectedNewModPath, newModPath)
		}
	}

	t.Run("v0 to v0", testGetImportPaths("mod/path", "0.0.0", "mod/path"))
	t.Run("v0 to v1", testGetImportPaths("mod/path", "1.2.3", "mod/path"))
	t.Run("v0 to v1", testGetImportPaths("mod/path", "1.2.3", "mod/path"))
	t.Run("v0 to v2", testGetImportPaths("mod/path", "2.3.4", "mod/path/v2"))
	t.Run("v2 to v3", testGetImportPaths("mod/path/v2", "3.4.5", "mod/path/v3"))
	t.Run("v1 to v0", testGetImportPaths("mod/path", "0.0.0", "mod/path"))
	t.Run("v2 to v0", testGetImportPaths("mod/path/v2", "0.0.0", "mod/path"))
}
