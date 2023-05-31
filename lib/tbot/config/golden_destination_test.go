/*
Copyright 2023 Gravitational, Inc.

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

package config

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/botfs"
)

type goldenDestination struct {
	goldenPath  string
	destination *DestinationDirectory
}

var goldenBasePath = filepath.Join("testdata", "goldenfs")
var fakeDestinationPath = "/test/destination"

func newGoldenDestination(t *testing.T) *goldenDestination {
	gd := &goldenDestination{
		goldenPath: filepath.Join(goldenBasePath, t.Name()),
		destination: &DestinationDirectory{
			Path:     filepath.Join(t.TempDir(), t.Name()),
			Symlinks: botfs.SymlinksInsecure,
			ACLs:     botfs.ACLOff,
		},
	}
	return gd
}

func (gd *goldenDestination) Assert(t *testing.T) {
	entries, err := os.ReadDir(gd.destination.Path)
	require.NoError(t, err)

	// Recurse through files in destination and compare with file in golden
	for _, entry := range entries {
		got, err := os.ReadFile(filepath.Join(gd.destination.Path, entry.Name()))
		require.NoError(t, err)
		want, err := os.ReadFile(filepath.Join(gd.goldenPath, entry.Name()))
		require.NoError(t, err)
		got = bytes.ReplaceAll(got, []byte(gd.destination.Path), []byte(fakeDestinationPath))
		require.Equal(t, string(want), string(got))
	}
}

func (gd *goldenDestination) Update(t *testing.T) {
	// Delete old files
	require.NoError(t, os.RemoveAll(gd.goldenPath))
	// Ensure testdata/goldenfs exists
	require.NoError(t, os.MkdirAll(gd.goldenPath, 0o777))
	// Copy the files/folders we *got* from running the test into the tests
	// golden folder
	err := exec.Command("cp", "-r", gd.destination.Path, goldenBasePath).Run()
	require.NoError(t, err)

	entries, err := os.ReadDir(gd.destination.Path)
	require.NoError(t, err)

	for _, entry := range entries {
		// TODO: support subdirs
		data, err := os.ReadFile(filepath.Join(gd.destination.Path, entry.Name()))
		require.NoError(t, err)
		data = bytes.ReplaceAll(data, []byte(gd.destination.Path), []byte(fakeDestinationPath))
		err = os.WriteFile(filepath.Join(gd.goldenPath, entry.Name()), data, 0o666)
		require.NoError(t, err)
	}
}
