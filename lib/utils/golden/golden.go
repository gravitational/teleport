/*
Copyright 2022 Gravitational, Inc.

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

// Golden files are a convenient way of storing data that we want to assert in
// unit tests. They are stored under the `testdata/` directory in a directory
// based on the name of the test. They are especially useful for storing large
// pieces of data that can be unwieldy to embed directly into your test tables.
//
// The convenience factor comes from the update mode which causes the tests to
// write data, rather than assert against it. This allows expected outputs
// to be updated easily when the underlying implementation is adjusted.
// This mode can be enabled by setting `GOLDEN_UPDATE=1` when running the tests
// you wish to update.
//
// Usage:
//
// Golden is ideal for testing the results of marshaling, or units that output
// large amounts of data to stdout or a file:
//
// 	func TestMarshalFooStruct(t *testing.T) {
//		got, err := json.Marshal(FooStruct{Some: "Data"})
//		require.NoError(t, err)
//
//		if golden.Update() {
//			golden.Set(t, got)
//		}
//		require.Equal(t, golden.Get(t), got)
//  }
//
// It is possible to have multiple golden files per test using `GetNamed` and
// `SetNamed`. This is useful for cases where your unit under test produces
// multiple pieces of output e.g stdout and stderr:
//
// 	func TestFooCommand(t *testing.T) {
//		stdoutBuf := new(bytes.Buffer)
//		stderrBuf := new(bytes.Buffer)
//
//		FooCommand(stdoutBuf, stderrBuf)
//
//		stdout := stdoutBuf.Bytes()
//		stderr := stderrBuf.Bytes()
//
//		if golden.Update() {
//			golden.SetNamed(t, "stdout", stdout)
//			golden.SetNamed(t, "stderr", stderr)
//		}
//		require.Equal(t, golden.GetNamed(t, "stdout"), stdout)
//		require.Equal(t, golden.GetNamed(t, "stderr"), stderr)
//	}

package golden

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func pathForFile(t *testing.T, name string) string {
	pathComponents := []string{
		"testdata",
		t.Name(),
	}

	if name != "" {
		pathComponents = append(pathComponents, name)
	}

	return filepath.Join(pathComponents...) + ".golden"
}

// ShouldSet provides a boolean value that indicates if your code should then
// call `Set` or `SetNamed` to update the stored golden file value with new
// data.
func ShouldSet() bool {
	env := os.Getenv("GOLDEN_UPDATE")
	should, _ := strconv.ParseBool(env)
	return should
}

// SetNamed writes the supplied data to a named golden file for the current
// test.
func SetNamed(t *testing.T, name string, data []byte) {
	p := pathForFile(t, name)
	dir := filepath.Dir(p)

	err := os.MkdirAll(dir, 0o755)
	require.NoError(t, err)

	err = os.WriteFile(p, data, 0o644)
	require.NoError(t, err)
}

// Set writes the supplied data to the golden file for the current test.
func Set(t *testing.T, data []byte) {
	SetNamed(t, "", data)
}

// GetNamed returns the contents of a named golden file for the current test. If
// the specified golden file does not exist for the test, the test will be
// failed.
func GetNamed(t *testing.T, name string) []byte {
	p := pathForFile(t, name)
	data, err := os.ReadFile(p)
	require.NoError(t, err)

	return data
}

// Get returns the contents of the golden file for the current test. If there is
// no golden file for the test, the test will be failed.
func Get(t *testing.T) []byte {
	return GetNamed(t, "")
}
