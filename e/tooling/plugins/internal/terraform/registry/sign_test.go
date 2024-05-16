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
package registry

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport-plugins/tooling/internal/filename"
)

const (
	contentFilename = "major-generals-song"
	content         = "I am the very model of a modern Major-General\nI've information Animal, Vegetable and Mineral."
)

func newPackage(t *testing.T, timestamp time.Time, version, system, arch string) string {
	v, err := semver.NewVersion(version)
	require.NoError(t, err)

	info := filename.Info{
		Type:    "terraform-provider",
		Version: *v,
		OS:      system,
		Arch:    arch,
	}
	filename := filepath.Join(t.TempDir(), info.Filename(".tar.gz"))

	f, err := os.Create(filename)
	require.NoError(t, err)
	defer f.Close()

	compressor := gzip.NewWriter(f)
	defer compressor.Close()

	tarwriter := tar.NewWriter(compressor)
	defer tarwriter.Close()

	err = tarwriter.WriteHeader(&tar.Header{
		Name:    contentFilename,
		Size:    int64(len(content)),
		Mode:    0755,
		ModTime: timestamp,
	})
	require.NoError(t, err)

	_, err = tarwriter.Write([]byte(content))
	require.NoError(t, err)

	return filename
}

func newKey(t *testing.T) *openpgp.Entity {
	entity, err := openpgp.NewEntity("testing", "test key", "root@example.com", nil)
	require.NoError(t, err)
	return entity
}

func TestRepackProvider(t *testing.T) {
	signer := newKey(t)
	timestamp := time.Now()
	srcPkg := newPackage(t, timestamp, "1.2.3", "linux", "arm")
	dstDir := t.TempDir()

	result, err := RepackProvider(dstDir, srcPkg, signer)
	require.NoError(t, err)
	require.Equal(t, semver.Version{Major: 1, Minor: 2, Patch: 3}, result.Version)
	require.Equal(t, "linux", result.OS)
	require.Equal(t, "arm", result.Arch)

	t.Run("Signature", func(t *testing.T) {
		keyring := openpgp.EntityList{signer}
		shafile, err := os.Open(result.Sum)
		require.NoError(t, err)
		defer shafile.Close()

		sigfile, err := os.Open(result.Sig)
		require.NoError(t, err)
		defer sigfile.Close()

		actualSigner, err := openpgp.CheckDetachedSignature(keyring, shafile, sigfile, nil)
		require.NoError(t, err)
		require.Equal(t, signer.PrivateKey.KeyId, actualSigner.PrivateKey.KeyId)
	})

	t.Run("Content", func(t *testing.T) {
		zipFile, err := zip.OpenReader(result.Zip)
		require.NoError(t, err)
		defer zipFile.Close()

		require.Len(t, zipFile.File, 1)
		f := zipFile.File[0]
		require.Equal(t, contentFilename, f.Name)
		require.Equal(t, fs.FileMode(0755), f.Mode())
		require.Equal(t, uint64(len(content)), f.UncompressedSize64)

		expectedTimestamp := timestamp.Round(time.Second)
		require.True(t, expectedTimestamp.Equal(f.Modified), "Expected %s == %s", expectedTimestamp, f.Modified)

		body, err := f.Open()
		require.NoError(t, err)
		defer body.Close()

		actualContent, err := io.ReadAll(body)
		require.NoError(t, err)
		require.Equal(t, content, string(actualContent))
	})
}
