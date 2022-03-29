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

	"github.com/gravitational/teleport/build.assets/gomod"

	"github.com/coreos/go-semver/semver"
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
	updatedGoFile := `package main

import "mod/path/v2"
import "other/mod/path"
import (
	"mod/path/v2"
	alias "mod/path/v2"

	"other/mod/path"
)
`

	// Create a dummy go module with go.mod and main.go file
	pkgDir := t.TempDir()
	writeFile(t, pkgDir, "go.mod", newGoModFileString("pkg"))

	// Run main.go file through the update function
	goFilePath := writeFile(t, pkgDir, "main.go", goFile)
	addRollBack := testRollBack(t, goFilePath, goFile)
	err := updateGoPkgs(pkgDir, "mod/path", "mod/path/v2", nil, addRollBack)
	require.NoError(t, err)
	readAndCompareFile(t, goFilePath, updatedGoFile)

	// Run updated main.go file through update function
	goFilePath = writeFile(t, pkgDir, "main.go", updatedGoFile)
	addRollBack = testRollBack(t, goFilePath, updatedGoFile)
	err = updateGoPkgs(pkgDir, "mod/path/v2", "mod/path", nil, addRollBack)
	require.NoError(t, err)
	readAndCompareFile(t, goFilePath, goFile)
}

func TestUpdateProtoFiles(t *testing.T) {
	protoFile := `syntax = "proto3";
package proto;
import "mod/path/types.proto";
message Example {
	types.Type field1 = 1 [
		(gogoproto.casttype) = "mod/path/types.Traits"
	];
	types.Type field2 = 2 [
		(gogoproto.customtype) = "mod/path/types.Traits"
	];
}
`

	// Only update casttype and customtype options.
	updatedProtoFile := `syntax = "proto3";
package proto;
import "mod/path/types.proto";
message Example {
	types.Type field1 = 1 [
		(gogoproto.casttype) = "mod/path/v2/types.Traits"
	];
	types.Type field2 = 2 [
		(gogoproto.customtype) = "mod/path/v2/types.Traits"
	];
}
`

	// Write proto file to disk
	dir := t.TempDir()

	// Run proto file through update function
	protoFilePath := writeFile(t, dir, "proto.proto", protoFile)
	addRollBack := testRollBack(t, protoFilePath, protoFile)
	err := updateProtoFiles(dir, "mod/path", "mod/path/v2", addRollBack)
	require.NoError(t, err)
	readAndCompareFile(t, protoFilePath, updatedProtoFile)

	// Run updated proto file through update function
	protoFilePath = writeFile(t, dir, "proto.proto", updatedProtoFile)
	addRollBack = testRollBack(t, protoFilePath, updatedProtoFile)
	err = updateProtoFiles(dir, "mod/path/v2", "mod/path", addRollBack)
	require.NoError(t, err)
	readAndCompareFile(t, protoFilePath, protoFile)
}

func TestUpdateGoModulePath(t *testing.T) {
	testUpdate := func(oldModPath, newModPath, newModVersion, oldModFile, expectedNewModFile string) func(t *testing.T) {
		return func(t *testing.T) {
			// Write mod file to disk
			modDir := t.TempDir()
			modFilePath := writeFile(t, modDir, "go.mod", oldModFile)

			addRollBack := testRollBack(t, modFilePath, oldModFile)

			// Run the mod file through the update function
			err := updateGoModFile(modDir, oldModPath, newModPath, newModVersion, addRollBack)
			require.NoError(t, err)

			// Read the updated mod file from disk and compare it to the expected mod file
			readAndCompareFile(t, modFilePath, expectedNewModFile)
		}
	}

	t.Run("updated module in header", testUpdate("go/mod/header", "updated/go/mod/header", "1.2.3",
		newGoModFileString("go/mod/header"),
		newGoModFileString("updated/go/mod/header"),
	))

	t.Run("updated module in statements", testUpdate("mod/path", "mod/path/v2", "1.2.3",
		newGoModFileString("go/mod/header", requireStatement("mod/path", "0.1.2")),
		newGoModFileString("go/mod/header", requireStatement("mod/path/v2", "1.2.3")),
	))

	t.Run("updated module not in mod file", testUpdate("mod/path", "mod/path/v2", "1.2.3",
		newGoModFileString("go/mod/header", requireStatement("other/mod/path", "0.1.2")),
		newGoModFileString("go/mod/header", requireStatement("other/mod/path", "0.1.2")),
	))

	// Create a go mod file with every type of go mod statement and
	// test that every statement and the header gets updated properly.
	testUpdateAllStatements := func(oldModPath, oldVersion, newVersion string) func(*testing.T) {
		oldModFile := newGoModFileString(oldModPath, allGoModStatements(oldModPath, oldVersion)...)
		newModPath := getNewModImportPath(oldModPath, semver.New(newVersion))
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

func TestGetImportPaths(t *testing.T) {
	testGetImportPaths := func(currentModPath, newVersion, expectedNewModPath string) func(t *testing.T) {
		return func(t *testing.T) {
			// Write mod file to disk
			modDir := t.TempDir()
			writeFile(t, modDir, "go.mod", newGoModFileString(currentModPath))

			// Get import paths using the mod file in disk
			oldModPath, err := gomod.GetImportPath(modDir)
			require.NoError(t, err)
			newModPath := getNewModImportPath(oldModPath, semver.New(newVersion))

			// Compare paths to expected results
			require.Equal(t, currentModPath, oldModPath)
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

// Write a file to disk and return the file path for testing
func writeFile(t *testing.T, dir, name, data string) string {
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(data), fs.ModePerm))
	return path
}

// Read a file and compare it to some expected file data for testing
func readAndCompareFile(t *testing.T, path, expectedData string) {
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, expectedData, string(data))
}

// Setup and test rollback functionality
func testRollBack(t *testing.T, filePath, expectedData string) addRollBackFunc {
	return func(rollBack rollBackFunc) {
		t.Cleanup(func() {
			err := rollBack()
			require.NoError(t, err)
			readAndCompareFile(t, filePath, expectedData)
		})
	}
}

// Create a go mod file as a string for testing
func newGoModFileString(modPath string, goModStatements ...string) string {
	header := fmt.Sprintf("module %v\n\ngo 1.15", modPath)
	return strings.Join(append([]string{header}, goModStatements...), "\n\n") + "\n"
}

// Helper functions for creating go mod statements for testing
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
