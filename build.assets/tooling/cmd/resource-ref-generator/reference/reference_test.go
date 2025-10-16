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
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// This test reads the golden files at the destination directory and compares
// the generated resource reference docs with them. To regenerate the golden
// files, delete the destination directory (reference/testdata/dest) and run the
// test again.
func TestGenerate(t *testing.T) {
	tdPath := "testdata"
	baseConf := GeneratorConfig{
		Resources: []ResourceConfig{
			{
				TypeName:    "DatabaseV3",
				PackageName: "typestest",
				NameInDocs:  "Database v3",
				KindValue:   "database",
			},
			{
				TypeName:    "DatabaseServerV3",
				PackageName: "typestest",
				NameInDocs:  "Database Server v3",
				KindValue:   "db_server",
			},
			{
				TypeName:    "AppV3",
				PackageName: "typestest",
				NameInDocs:  "Application v3",
				KindValue:   "application",
			},
			{
				TypeName:    "AppServerV3",
				PackageName: "typestest",
				NameInDocs:  "App Server v3",
				KindValue:   "app_server",
			},
			{
				TypeName:    "Bot",
				PackageName: "machineidv1",
				NameInDocs:  "Bot v1",
				KindValue:   "bot",
			},
		},
		SourcePath: filepath.Join(
			tdPath, "src",
		),
		DestinationDirectory: "dest",
	}

	goldenConf := baseConf
	goldenConf.DestinationDirectory = filepath.Join(tdPath, "dest")

	dirExpected, err := os.ReadDir(goldenConf.DestinationDirectory)

	switch {
	// Recreate the golden file directory if it is missing.
	case errors.Is(err, os.ErrNotExist):
		if err := os.Mkdir(goldenConf.DestinationDirectory, 0777); err != nil {
			t.Fatal(err)
		}
		if err := Generate(goldenConf); err != nil {
			t.Fatal(err)
		}
		return
	case err != nil:
		t.Fatal(err)
	}

	tmp := t.TempDir()
	tmpConf := baseConf
	tmpConf.DestinationDirectory = filepath.Join(tmp, "dest")

	if err := os.Mkdir(tmpConf.DestinationDirectory, 0777); err != nil {
		t.Fatal(err)
	}

	if err := Generate(tmpConf); err != nil {
		t.Fatal(err)
	}

	fileActual, err := os.Open(tmpConf.DestinationDirectory)
	if err != nil {
		t.Fatal(err)
	}

	dirActual, err := fileActual.Readdir(-1)
	if err != nil {
		t.Fatal(err)
	}

	expectedFiles := make(map[string]struct{})
	for _, f := range dirExpected {
		expectedFiles[f.Name()] = struct{}{}
	}

	actualFiles := make(map[string]struct{})
	for _, f := range dirActual {
		actualFiles[f.Name()] = struct{}{}
	}

	for f := range actualFiles {
		if _, ok := expectedFiles[f]; !ok {
			t.Fatalf(
				"file %v created after running the generator but is not in %v",
				f,
				tmpConf.DestinationDirectory,
			)
		}
	}
	for f := range expectedFiles {
		if _, ok := actualFiles[f]; !ok {
			t.Fatalf(
				"file %v in %v was not created after running the generator",
				f,
				goldenConf.DestinationDirectory,
			)
		}
	}

	// Actual file names and expected file names match, so we can compare
	// each file.
	for f := range actualFiles {
		actual, err := os.Open(filepath.Join(tmpConf.DestinationDirectory, f))
		if err != nil {
			t.Fatal(err)
		}
		actualContent, err := io.ReadAll(actual)
		if err != nil {
			t.Fatal(err)
		}

		expected, err := os.Open(filepath.Join(goldenConf.DestinationDirectory, f))
		if err != nil {
			t.Fatal(err)
		}

		expectedContent, err := io.ReadAll(expected)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, string(expectedContent), string(actualContent))
	}
}
