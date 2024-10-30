/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package scanner

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/testing/protocmp"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
	scantestdata "github.com/gravitational/teleport/lib/secretsscanner/scanner/testdata"
)

var (
	deviceID = uuid.NewString()
)

func TestNewScanner(t *testing.T) {
	tests := []struct {
		name         string
		keysGen      func(t *testing.T, path string) []*accessgraphsecretsv1pb.PrivateKey
		skipTestDir  bool
		assertResult func(t *testing.T, got []*accessgraphsecretsv1pb.PrivateKey)
	}{
		{
			name:    "encrypted keys",
			keysGen: writeEncryptedKeys,
		},
		{
			name:    "unencrypted keys",
			keysGen: writeUnEncryptedKeys,
		},
		{
			name:    "encryptedKey without public key file",
			keysGen: writeEncryptedKeyWithoutPubFile,
		},
		{
			name:    "invalid keys",
			keysGen: writeInvalidKeys,
		},
		{
			name:        "skip test dir keys",
			keysGen:     writeUnEncryptedKeys,
			skipTestDir: true,
			assertResult: func(t *testing.T, got []*accessgraphsecretsv1pb.PrivateKey) {
				require.Empty(t, got, "ScanPrivateKeys with skip test dir should return empty keys")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			expect := tt.keysGen(t, dir)

			var skipPaths []string
			if tt.skipTestDir {
				// skip the test directory.
				skipPaths = []string{filepath.Join(dir, "*")}
				// the expected keys should be nil since the test directory is skipped.
				expect = nil
			}

			s, err := New(Config{
				Dirs:      []string{dir},
				SkipPaths: skipPaths,
			})
			require.NoError(t, err)

			keys := s.ScanPrivateKeys(context.Background(), deviceID)
			var got []*accessgraphsecretsv1pb.PrivateKey
			for _, key := range keys {
				got = append(got, key.Key)
			}

			// Sort the keys by name for comparison.
			sortPrivateKeys(expect)
			sortPrivateKeys(got)

			if tt.assertResult != nil {
				tt.assertResult(t, got)
			}

			diff := cmp.Diff(expect, got, protocmp.Transform())
			require.Empty(t, diff, "ScanPrivateKeys keys mismatch (-got +want)")
		})
	}
}

func sortPrivateKeys(keys []*accessgraphsecretsv1pb.PrivateKey) {
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Metadata.Name < keys[j].Metadata.Name
	})
}

func writeEncryptedKeys(t *testing.T, dir string) []*accessgraphsecretsv1pb.PrivateKey {
	t.Helper()
	var expectedKeys []*accessgraphsecretsv1pb.PrivateKey
	// Write encrypted keys to the directory.
	for _, key := range scantestdata.PEMEncryptedKeys {
		err := os.Mkdir(filepath.Join(dir, key.Name), 0o777)
		require.NoError(t, err)

		filePath := filepath.Join(dir, key.Name, key.Name)
		err = os.WriteFile(filePath, key.PEMBytes, 0o666)
		require.NoError(t, err)

		s, err := ssh.ParsePrivateKeyWithPassphrase(key.PEMBytes, []byte(key.EncryptionKey))
		require.NoError(t, err)

		if !key.IncludesPublicKey {
			pubFilePath := filePath + ".pub"
			authorizedKeyBytes := ssh.MarshalAuthorizedKey(s.PublicKey())
			require.NoError(t, os.WriteFile(pubFilePath, authorizedKeyBytes, 0o666))
		}

		fingerprint := ssh.FingerprintSHA256(s.PublicKey())

		mode := accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED
		if !key.IncludesPublicKey {
			mode = accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE
		}

		key, err := accessgraph.NewPrivateKeyWithName(
			privateKeyNameGen(filePath, deviceID, fingerprint),
			&accessgraphsecretsv1pb.PrivateKeySpec{
				PublicKeyFingerprint: fingerprint,
				DeviceId:             deviceID,
				PublicKeyMode:        mode,
			},
		)
		require.NoError(t, err)

		expectedKeys = append(expectedKeys, key)
	}

	return expectedKeys
}

func writeUnEncryptedKeys(t *testing.T, dir string) []*accessgraphsecretsv1pb.PrivateKey {
	t.Helper()
	var expectedKeys []*accessgraphsecretsv1pb.PrivateKey

	for name, key := range scantestdata.PEMBytes {
		err := os.Mkdir(filepath.Join(dir, name), 0o777)
		require.NoError(t, err)

		filePath := filepath.Join(dir, name, name)
		err = os.WriteFile(filePath, key, 0o666)
		require.NoError(t, err)

		s, err := ssh.ParsePrivateKey(key)
		require.NoError(t, err)

		fingerprint := ssh.FingerprintSHA256(s.PublicKey())

		key, err := accessgraph.NewPrivateKeyWithName(
			privateKeyNameGen(filePath, deviceID, fingerprint),
			&accessgraphsecretsv1pb.PrivateKeySpec{
				PublicKeyFingerprint: fingerprint,
				DeviceId:             deviceID,
				PublicKeyMode:        accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED,
			},
		)
		require.NoError(t, err)

		expectedKeys = append(expectedKeys, key)
	}

	return expectedKeys
}

func writeEncryptedKeyWithoutPubFile(t *testing.T, dir string) []*accessgraphsecretsv1pb.PrivateKey {
	t.Helper()

	// Write encrypted keys to the directory.
	rawKey := scantestdata.PEMEncryptedKeys[0]
	err := os.Mkdir(filepath.Join(dir, rawKey.Name), 0o777)
	require.NoError(t, err)

	filePath := filepath.Join(dir, rawKey.Name, rawKey.Name)
	err = os.WriteFile(filePath, rawKey.PEMBytes, 0o666)
	require.NoError(t, err)

	key, err := accessgraph.NewPrivateKeyWithName(
		privateKeyNameGen(filePath, deviceID, ""),
		&accessgraphsecretsv1pb.PrivateKeySpec{
			PublicKeyFingerprint: "",
			DeviceId:             deviceID,
			PublicKeyMode:        accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED,
		},
	)
	require.NoError(t, err)

	return []*accessgraphsecretsv1pb.PrivateKey{key}
}

func writeInvalidKeys(t *testing.T, dir string) []*accessgraphsecretsv1pb.PrivateKey {
	t.Helper()

	// Write invalid keys to the directory.
	for path, keyBytes := range scantestdata.InvalidKeysBytes {
		err := os.Mkdir(filepath.Join(dir, path), 0o777)
		require.NoError(t, err)

		filePath := filepath.Join(dir, path, path)
		err = os.WriteFile(filePath, keyBytes, 0o666)
		require.NoError(t, err)
	}

	return nil
}
