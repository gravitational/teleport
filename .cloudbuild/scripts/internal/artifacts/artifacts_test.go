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

package artifacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, data []byte, path ...string) string {
	t.Helper()
	filePath := filepath.Join(path...)
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0777))
	require.NoError(t, os.WriteFile(filePath, data, 0644))
	return filePath
}

func touch(t *testing.T, path ...string) string {
	t.Helper()
	return write(t, []byte{}, path...)
}

func TestValidatePatterns(t *testing.T) {
	workspace := t.TempDir()

	t.Run("Patterns are expanded", func(t *testing.T) {
		patterns := []string{"alpha", "nested/beta", "*"}

		expected := []string{
			filepath.Join(workspace, "alpha"),
			filepath.Join(workspace, "nested", "beta"),
			filepath.Join(workspace, "*"),
		}

		actual, err := ValidatePatterns(workspace, patterns)
		require.NoError(t, err)

		require.ElementsMatch(t, actual, expected)
	})

	t.Run("Paths are canonicalised", func(t *testing.T) {
		patterns := []string{
			"nested/../alpha",
			"./beta",
		}

		expected := []string{
			filepath.Join(workspace, "alpha"),
			filepath.Join(workspace, "beta"),
		}

		actual, err := ValidatePatterns(workspace, patterns)
		require.NoError(t, err)

		require.ElementsMatch(t, actual, expected)
	})

	t.Run("Paths outside workspace fail", func(t *testing.T) {
		t.Run("fully-qualified path", func(t *testing.T) {
			_, err := ValidatePatterns(workspace, []string{t.TempDir()})
			require.Error(t, err)
		})

		t.Run("relative path", func(t *testing.T) {
			target := "../../root/**/*"
			_, err := ValidatePatterns(workspace, []string{target})
			require.Error(t, err)
		})
	})
}

func TestFindArtifacts(t *testing.T) {
	workspace := t.TempDir()

	alpha := touch(t, workspace, "alpha.yaml")
	beta := touch(t, workspace, "beta.yaml")
	gamma := touch(t, workspace, "gamma.some-other-extension")
	delta := touch(t, workspace, "nested", "delta.yaml")
	epsilon := touch(t, workspace, "nested", "epsilon.yaml")
	zeta := touch(t, workspace, "nested", "deeply", "zeta")
	eta := touch(t, workspace, "nested", "very", "deeply", "eta.yaml")

	t.Run("root-dir", func(t *testing.T) {
		patterns := []string{filepath.Join(workspace, "*")}
		actual := find(patterns)
		expected := []string{alpha, beta, gamma}
		require.ElementsMatch(t, expected, actual)
	})

	t.Run("prefix", func(t *testing.T) {
		patterns := []string{
			filepath.Join(workspace, "nested/*"),
			filepath.Join(workspace, "nested/*/*"),
			filepath.Join(workspace, "nested/*/*/*"),
		}
		actual := find(patterns)
		expected := []string{delta, epsilon, zeta, eta}
		require.ElementsMatch(t, expected, actual)
	})

	t.Run("suffix", func(t *testing.T) {
		patterns := []string{
			filepath.Join(workspace, "*.yaml"),
			filepath.Join(workspace, "*/*.yaml"),
			filepath.Join(workspace, "*/*/*.yaml"),
			filepath.Join(workspace, "*/*/*/*.yaml"),
		}
		actual := find(patterns)
		expected := []string{alpha, beta, delta, epsilon, eta}
		require.ElementsMatch(t, expected, actual)
	})

	t.Run("infix", func(t *testing.T) {
		patterns := []string{filepath.Join(workspace, "*/very/deeply/*")}
		actual := find(patterns)
		expected := []string{eta}
		require.ElementsMatch(t, expected, actual)
	})
}

func TestUpload(t *testing.T) {
	workspace := t.TempDir()

	ctx := context.Background()
	bucket := new(mockBucket)

	mockArtifact := func(content []byte, path ...string) (string, *bytes.Buffer) {
		// Create the source file that the upload() function should find
		// and upload
		src := write(t, content, path...)

		// Rig up a mock that will receive the "uploaded" content
		dst := &bytes.Buffer{}
		obj := &mockStorageObject{}
		obj.On("NewWriter", mock.Anything).
			Return(&closeWrapper{Writer: dst}).
			Once()

		bucket.On("Object", "artifacts/"+path[len(path)-1]).
			Return(obj).
			Once()
		return src, dst
	}

	// Given a configured artifact list and a mocked-out upload receiver...
	const alphaContent = "I am the very model of a modern major-general"
	alphaSrc, alphaDst := mockArtifact([]byte(alphaContent), workspace, "nested", "alpha.txt")

	const betaContent = "I've information vegetable, animal, and mineral"
	betaSrc, betaDst := mockArtifact([]byte(betaContent), workspace, "beta.txt")

	// When I upload artifact files...
	files := []string{alphaSrc, betaSrc}
	err := upload(ctx, bucket, "artifacts", files)

	// Expect that the upload succeeds
	require.NoError(t, err)

	// And that the file content was written to the objects
	require.Equal(t, alphaContent, alphaDst.String())
	require.Equal(t, betaContent, betaDst.String())
}

func TestFailedUpload(t *testing.T) {
	workspace := t.TempDir()

	ctx := context.Background()
	bucket := new(mockBucket)

	mockArtifact := func(content []byte, dst io.WriteCloser, path ...string) string {
		// Create the source item that the upload() function should find
		// and upload
		src := write(t, content, path...)

		// Rig up a mock that will receive the "uploaded" content
		obj := &mockStorageObject{}
		obj.On("NewWriter", mock.Anything).
			Return(dst).
			Once()

		bucket.On("Object", "artifacts/"+path[len(path)-1]).
			Return(obj).
			Once()
		return src
	}

	mockHappyPath := func(content []byte, path ...string) (string, *bytes.Buffer) {
		// Mock up an uploader that will just succeed
		dst := &bytes.Buffer{}
		src := mockArtifact(content, &closeWrapper{Writer: dst}, path...)
		return src, dst
	}

	// Given a configured artifact list and a mocked-out upload receiver...
	const alphaContent = "I am the very model of a modern major-general"
	alphaSrc, alphaDst := mockHappyPath([]byte(alphaContent), workspace, "nested", "alpha.txt")

	// .. where one of the uploads will fail on write...
	const betaContent = "I've information vegetable, animal, and mineral"
	betaErr := errors.New("Spontaneous failure")
	betaDst := &mockWriter{}
	betaDst.
		On("Write", mock.AnythingOfType("[]uint8")).
		Return(0, betaErr)
	betaSrc := mockArtifact([]byte(betaContent), betaDst, workspace, "fail-on-write.txt")

	// .. and another will fail on close...
	const gammaContent = "I know the kings of England, and I quote the fights Historical"
	gammaErr := errors.New("Fail on close")
	gammaDst := &mockWriter{}
	gammaDst.On("Write", mock.AnythingOfType("[]uint8"))
	gammaDst.On("Close").Return(gammaErr)
	gammaSrc := mockArtifact([]byte(gammaContent), gammaDst, workspace, "fail-on-close.txt")

	// .. and another flat-out-fails to exist...
	const deltaContent = "From Marathon to Waterloo, in order categorical"
	deltaSrc := mockArtifact([]byte(deltaContent), &mockWriter{}, workspace, "no", "such", "file.txt")
	require.NoError(t, os.Remove(deltaSrc))

	// with a final entry that should succeed
	const epsilonContent = "I'm very well acquainted, too, with matters Mathematical"
	epsilonSrc, epsilonDst := mockHappyPath([]byte(epsilonContent), workspace, "epsilon.txt")

	// When I actually try to upload these files...
	files := []string{alphaSrc, betaSrc, gammaSrc, deltaSrc, epsilonSrc}
	err := upload(ctx, bucket, "artifacts", files)

	// Expect that 3 out of the 5 uploads failed (implying an overall failure)
	require.Error(t, err)
	errs := err.(*multierror.Error)
	require.Equal(t, 3, errs.Len())

	// ...but that the rest are still uploaded
	require.Equal(t, alphaContent, alphaDst.String())
	require.Equal(t, epsilonContent, epsilonDst.String())
}
