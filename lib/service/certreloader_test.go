/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package service

import (
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestCertReloader(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc                   string
		certsUpdate            func(t *testing.T, certs []servicecfg.KeyPairPath)
		certsReloadErrorAssert require.ErrorAssertionFunc
		certsAssert            func(t *testing.T, before []tls.Certificate, now []tls.Certificate)
	}{
		{
			desc: "c0 and c1 certs do not change without an update",
			certsUpdate: func(t *testing.T, certs []servicecfg.KeyPairPath) {
				// No update.
			},
			certsReloadErrorAssert: require.NoError,
			certsAssert: func(t *testing.T, before []tls.Certificate, after []tls.Certificate) {
				// No cert has been updated.
				require.Len(t, before, 2)
				require.Len(t, after, 2)
				require.Equal(t, before[0], after[0])
				require.Equal(t, before[1], after[1])
			},
		},
		{
			desc: "c0 cert does change with an update",
			certsUpdate: func(t *testing.T, certs []servicecfg.KeyPairPath) {
				// Update c0 cert.
				key, crt := newCertKeyPair(t)
				write(t, certs[0].PrivateKey, key)
				write(t, certs[0].Certificate, crt)
			},
			certsReloadErrorAssert: require.NoError,
			certsAssert: func(t *testing.T, before []tls.Certificate, after []tls.Certificate) {
				// Only c0 has been updated.
				require.Len(t, before, 2)
				require.Len(t, after, 2)
				require.NotEqual(t, before[0], after[0])
				require.Equal(t, before[1], after[1])
			},
		},
		{
			desc: "c0 and c1 certs do change with an update",
			certsUpdate: func(t *testing.T, certs []servicecfg.KeyPairPath) {
				// Update c0 cert.
				key, crt := newCertKeyPair(t)
				write(t, certs[0].PrivateKey, key)
				write(t, certs[0].Certificate, crt)

				// Update c1 cert.
				key, crt = newCertKeyPair(t)
				write(t, certs[1].PrivateKey, key)
				write(t, certs[1].Certificate, crt)
			},
			certsReloadErrorAssert: require.NoError,
			certsAssert: func(t *testing.T, before []tls.Certificate, after []tls.Certificate) {
				// Both certs have been updated.
				require.Len(t, before, 2)
				require.Len(t, after, 2)
				require.NotEqual(t, before[0], after[0])
				require.NotEqual(t, before[1], after[1])
			},
		},
		{
			desc: "c0 and c1 certs do not change with an incomplete update",
			certsUpdate: func(t *testing.T, certs []servicecfg.KeyPairPath) {
				// Update c0 cert.
				key, crt := newCertKeyPair(t)
				write(t, certs[0].PrivateKey, key)
				write(t, certs[0].Certificate, crt)

				// Update only c1 key.
				key, _ = newCertKeyPair(t)
				write(t, certs[1].PrivateKey, key)
			},
			certsReloadErrorAssert: require.Error,
			certsAssert: func(t *testing.T, before []tls.Certificate, after []tls.Certificate) {
				// No cert has been updated.
				require.Len(t, before, 2)
				require.Len(t, after, 2)
				require.Equal(t, before[0], after[0])
				require.Equal(t, before[1], after[1])
			},
		},
		{
			desc: "c0 cert does not change during an ongoing update",
			certsUpdate: func(t *testing.T, certs []servicecfg.KeyPairPath) {
				// Update c0 key, and partially update c0 cert.
				key, crt := newCertKeyPair(t)
				write(t, certs[0].PrivateKey, key)
				write(t, certs[0].Certificate, crt[0:len(crt)/2])
			},
			certsReloadErrorAssert: require.Error,
			certsAssert: func(t *testing.T, before []tls.Certificate, after []tls.Certificate) {
				// No cert has been updated.
				require.Len(t, before, 2)
				require.Len(t, after, 2)
				require.Equal(t, before[0], after[0])
				require.Equal(t, before[1], after[1])
			},
		},
		{
			desc: "c0 and c1 certs do not change if one of them is corrupted",
			certsUpdate: func(t *testing.T, certs []servicecfg.KeyPairPath) {
				// Corrupt c0 cert key.
				f, err := os.OpenFile(certs[0].PrivateKey, os.O_WRONLY, 0600)
				require.NoError(t, err)
				_, err = f.WriteAt([]byte{1, 2, 3, 4, 5, 6, 7, 8}, 0)
				require.NoError(t, err)
				require.NoError(t, f.Sync())
				require.NoError(t, f.Close())
			},
			certsReloadErrorAssert: require.Error,
			certsAssert: func(t *testing.T, before []tls.Certificate, after []tls.Certificate) {
				// No cert has been updated.
				require.Len(t, before, 2)
				require.Len(t, after, 2)
				require.Equal(t, before[0], after[0])
				require.Equal(t, before[1], after[1])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			// Create empty certs and ensure they get cleaned up.
			certs := newCerts(t)

			// Start cert reloader.
			// Set the reload interval to 0 so that the reloading goroutine is not spawned.
			// This gives us more flexibility in the tests, so that we can call loadCertificates
			// when we want.
			cfg := CertReloaderConfig{
				KeyPairs:               certs,
				KeyPairsReloadInterval: 0,
			}
			certReloader := NewCertReloader(cfg)
			err := certReloader.Run(ctx)

			// Check that certificates load correctly in the synchronous (first) attempt.
			require.NoError(t, err)

			// Save certs before update.
			before := make([]tls.Certificate, len(certReloader.certificates))
			copy(before, certReloader.certificates)

			// Perform cert update.
			tc.certsUpdate(t, certs)

			// Perform cert reload.
			err = certReloader.loadCertificates(ctx)
			tc.certsReloadErrorAssert(t, err)

			// Perform certs assert, passing in the certs before & after the update.
			tc.certsAssert(t, before, certReloader.certificates)
		})
	}
}

// newCerts creates 2 certificate key pairs and returns
// the key pair paths to them.
func newCerts(t *testing.T) []servicecfg.KeyPairPath {
	dir := t.TempDir()
	certs := []servicecfg.KeyPairPath{
		{
			PrivateKey:  filepath.Join(dir, "c0.key"),
			Certificate: filepath.Join(dir, "c0.crt"),
		},
		{
			PrivateKey:  filepath.Join(dir, "c1.key"),
			Certificate: filepath.Join(dir, "c1.crt"),
		},
	}

	// Create c0 cert.
	key, crt := newCertKeyPair(t)
	write(t, certs[0].PrivateKey, key)
	write(t, certs[0].Certificate, crt)

	// Create c1 cert.
	key, crt = newCertKeyPair(t)
	write(t, certs[1].PrivateKey, key)
	write(t, certs[1].Certificate, crt)

	return certs
}

// newCertKeyPair creates a new cert.
func newCertKeyPair(t *testing.T) ([]byte, []byte) {
	entity := pkix.Name{
		Organization: []string{"teleport"},
		CommonName:   "teleport",
	}
	key, crt, err := tlsca.GenerateSelfSignedCA(entity, nil, time.Hour)
	require.NoError(t, err)
	return key, crt
}

// write rewrites the file with a new `content`.
func write(t *testing.T, filename string, content []byte) {
	err := os.WriteFile(filename, content, 0600)
	require.NoError(t, err)
}
