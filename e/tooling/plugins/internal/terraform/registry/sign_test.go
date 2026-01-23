package registry

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport.e/tooling/plugins/internal/filename"
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

func TestIsProviderTarball(t *testing.T) {
	tests := []struct {
		fn      string
		variant string
		want    bool
	}{
		{
			fn:      "/tmp/artifacts/terraform-provider-teleportmwi-v19.0.0-dev.noahmwitf.4-darwin-amd64-bin.tar.gz",
			variant: "mwi",
			want:    true,
		},
		{
			fn:      "/tmp/artifacts/terraform-provider-teleport-v19.0.0-dev.noahmwitf.4-darwin-amd64-bin.tar.gz",
			variant: "",
			want:    true,
		},
		{
			fn:      "/tmp/artifacts/terraform-provider-teleportmwi-v19.0.0-dev.noahmwitf.4-darwin-amd64-bin.tar.gz",
			variant: "",
			want:    false,
		},
		{
			fn:      "/tmp/artifacts/terraform-provider-teleport-v19.0.0-dev.noahmwitf.4-darwin-amd64-bin.tar.gz",
			variant: "mwi",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s - %s", tt.fn, tt.variant), func(t *testing.T) {
			got := IsProviderTarball(tt.fn, tt.variant)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestWriteMasterManifest(t *testing.T) {
	// Arrange
	ctx := context.Background()

	signer := newKey(t)

	inputContentForTest := "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03  terraform-provider-teleport_18.5.1_linux_amd64.zip\n" +
		"7a1e0952671520624d6786a3d6f14a6a5751950392348a2e846f6be036571950  terraform-provider-teleport_18.5.1_darwin_arm64.zip\n"

	sumsReader := strings.NewReader(inputContentForTest)

	manifestBuffer := new(bytes.Buffer)
	signatureBuffer := new(bytes.Buffer)

	// Act
	err := WriteMasterManifest(ctx, sumsReader, signer, manifestBuffer, signatureBuffer)

	// Assert
	require.NoError(t, err)
	require.Equal(t, inputContentForTest, manifestBuffer.String(), "manifest content must contain exactly what was read from input reader")

	// verify signature
	t.Run("Cryptographic verification", func(t *testing.T) {
		keyring := openpgp.EntityList{signer}

		actualSigner, err := openpgp.CheckDetachedSignature(
			keyring,
			bytes.NewReader(manifestBuffer.Bytes()),
			bytes.NewReader(signatureBuffer.Bytes()),
			nil,
		)

		require.NoError(t, err, "signature should be valid for the generated manifest")
		require.Equal(t, signer.PrivateKey.KeyId, actualSigner.PrivateKey.KeyId)
	})
}
