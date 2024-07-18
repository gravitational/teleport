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

package scan

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
	cryptosshtestdata "golang.org/x/crypto/ssh/testdata"
	"google.golang.org/protobuf/testing/protocmp"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
)

var (
	deviceID = uuid.NewString()
)

func TestNewScanner(t *testing.T) {
	tests := []struct {
		name    string
		keysGen func(t *testing.T, path string) []*accessgraphsecretsv1pb.PrivateKey
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			expect := tt.keysGen(t, dir)
			s, err := NewScanner(ScannerConfig{Dirs: []string{dir}})
			require.NoError(t, err)

			keys := s.ScanPrivateKeys(context.Background(), deviceID)
			var got []*accessgraphsecretsv1pb.PrivateKey
			for _, key := range keys {
				got = append(got, key.Key)
			}

			// Sort the keys by name for comparison.
			sortPrivateKeys(expect)
			sortPrivateKeys(got)

			diff := cmp.Diff(expect, got, protocmp.Transform())
			require.Empty(t, diff, "unexpected keys")

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
	for _, key := range cryptosshtestdata.PEMEncryptedKeys {
		err := os.Mkdir(filepath.Join(dir, key.Name), os.ModePerm)
		require.NoError(t, err)

		filePath := filepath.Join(dir, key.Name, key.Name)
		err = os.WriteFile(filePath, key.PEMBytes, os.ModePerm)
		require.NoError(t, err)

		s, err := ssh.ParsePrivateKeyWithPassphrase(key.PEMBytes, []byte(key.EncryptionKey))
		require.NoError(t, err)

		if !key.IncludesPublicKey {
			pubFilePath := filePath + ".pub"
			authorizedKeyBytes := ssh.MarshalAuthorizedKey(s.PublicKey())
			require.NoError(t, os.WriteFile(pubFilePath, authorizedKeyBytes, os.ModePerm))
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

	for name, key := range cryptosshtestdata.PEMBytes {
		err := os.Mkdir(filepath.Join(dir, name), os.ModePerm)
		require.NoError(t, err)

		filePath := filepath.Join(dir, name, name)
		err = os.WriteFile(filePath, key, os.ModePerm)
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
	rawKey := cryptosshtestdata.PEMEncryptedKeys[0]
	err := os.Mkdir(filepath.Join(dir, rawKey.Name), os.ModePerm)
	require.NoError(t, err)

	filePath := filepath.Join(dir, rawKey.Name, rawKey.Name)
	err = os.WriteFile(filePath, rawKey.PEMBytes, os.ModePerm)
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
