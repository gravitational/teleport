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
package service

import (
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestCertReloader(t *testing.T) {
	testCases := []struct {
		desc                   string
		certsUpdate            func(t *testing.T, certs *certFiles)
		certsReloadErrorAssert require.ErrorAssertionFunc
		certsAssert            func(t *testing.T, before []tls.Certificate, now []tls.Certificate)
	}{
		{
			desc: "c1 and c2 certs do not change without an update",
			certsUpdate: func(t *testing.T, certs *certFiles) {
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
			desc: "c1 cert does change with an update",
			certsUpdate: func(t *testing.T, certs *certFiles) {
				// Update c1 cert.
				key, crt := newCertKeyPair(t)
				write(t, certs.c1Key, key)
				write(t, certs.c1Crt, crt)
			},
			certsReloadErrorAssert: require.NoError,
			certsAssert: func(t *testing.T, before []tls.Certificate, after []tls.Certificate) {
				// Only c1 has been updated.
				require.Len(t, before, 2)
				require.Len(t, after, 2)
				require.NotEqual(t, before[0], after[0])
				require.Equal(t, before[1], after[1])
			},
		},
		{
			desc: "c1 and c2 certs do change with an update",
			certsUpdate: func(t *testing.T, certs *certFiles) {
				// Update c1 cert.
				key, crt := newCertKeyPair(t)
				write(t, certs.c1Key, key)
				write(t, certs.c1Crt, crt)

				// Update c2 cert.
				key, crt = newCertKeyPair(t)
				write(t, certs.c2Key, key)
				write(t, certs.c2Crt, crt)
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
			desc: "c1 and c2 certs do not change with an incomplete update",
			certsUpdate: func(t *testing.T, certs *certFiles) {
				// Update c1 cert.
				key, crt := newCertKeyPair(t)
				write(t, certs.c1Key, key)
				write(t, certs.c1Crt, crt)

				// Update only c2 key.
				key, _ = newCertKeyPair(t)
				write(t, certs.c2Key, key)
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
			desc: "c1 cert does not change during an ongoing update",
			certsUpdate: func(t *testing.T, certs *certFiles) {
				// Update c1 key, and partially update c1 cert.
				key, crt := newCertKeyPair(t)
				write(t, certs.c1Key, key)
				write(t, certs.c1Crt, crt[0:1024])
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
			desc: "c1 and c2 certs do not change if one of them is corrupted",
			certsUpdate: func(t *testing.T, certs *certFiles) {
				// Corrupt c1 cert key.
				_, err := certs.c1Key.WriteAt([]byte{1, 2, 3, 4, 5, 6, 7, 8}, 0)
				require.NoError(t, err)
				err = certs.c1Key.Sync()
				require.NoError(t, err)
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
			// Create empty certs and ensure they get cleaned up.
			certs := newCerts(t)
			defer certs.cleanup(t)

			// Start cert reloader.
			// Set the reload interval to 0 so that the reloading goroutine is not spawned.
			// This gives us more flexibility in the tests, so that we can call loadCertificates
			// when we want.
			cfg := CertReloaderConfig{
				KeyPairs:               certs.keyPairPaths(),
				KeyPairsReloadInterval: 0,
			}
			certReloader := NewCertReloader(cfg)
			err := certReloader.Run(context.TODO())

			// Check that certificates load correctly in the synchronous (first) attempt.
			require.NoError(t, err)

			// Save certs before update.
			before := make([]tls.Certificate, len(certReloader.certificates))
			copy(before, certReloader.certificates)

			// Perform cert update.
			tc.certsUpdate(t, &certs)

			// Perform cert reload.
			err = certReloader.loadCertificates()
			tc.certsReloadErrorAssert(t, err)

			// Perform certs assert, passing in the certs before & after the update.
			tc.certsAssert(t, before, certReloader.certificates)
		})
	}
}

// certFiles contains 2 certificate key pairs.
type certFiles struct {
	c1Key *os.File
	c1Crt *os.File
	c2Key *os.File
	c2Crt *os.File
}

// newCerts creates files for 2 certificate key pairs,
// generates 2 certificate key pairs, and writes them
// to the files created.
func newCerts(t *testing.T) certFiles {
	// Create files where key-pair certs will be written to.
	c1Key, err := os.CreateTemp("", "cert1_*.key")
	require.NoError(t, err)
	c1Crt, err := os.CreateTemp("", "cert1_*.crt")
	require.NoError(t, err)

	c2Key, err := os.CreateTemp("", "cert2_*.key")
	require.NoError(t, err)
	c2Crt, err := os.CreateTemp("", "cert2_*.crt")
	require.NoError(t, err)

	// Create key-pair certs and write them to the files.
	c1KeyBytes, c1CrtBytes := newCertKeyPair(t)
	write(t, c1Key, c1KeyBytes)
	write(t, c1Crt, c1CrtBytes)

	c2KeyBytes, c2CrtBytes := newCertKeyPair(t)
	write(t, c2Key, c2KeyBytes)
	write(t, c2Crt, c2CrtBytes)

	return certFiles{
		c1Key: c1Key,
		c1Crt: c1Crt,
		c2Key: c2Key,
		c2Crt: c2Crt,
	}
}

// keyPairPaths returns the paths to both certs.
func (c *certFiles) keyPairPaths() []KeyPairPath {
	return []KeyPairPath{
		{
			PrivateKey:  c.c1Key.Name(),
			Certificate: c.c1Crt.Name(),
		},
		{
			PrivateKey:  c.c2Key.Name(),
			Certificate: c.c2Crt.Name(),
		},
	}
}

// cleanup deletes all cert files.
func (c *certFiles) cleanup(t *testing.T) {
	files := []*os.File{c.c1Key, c.c1Crt, c.c2Key, c.c2Crt}
	for _, f := range files {
		err := os.Remove(f.Name())
		require.NoError(t, err)
	}
}

// newCertKeyPair creates a new cert.
func newCertKeyPair(t *testing.T) ([]byte, []byte) {
	entity := pkix.Name{
		Organization: []string{"teleport"},
		CommonName:   "teleport",
	}
	key, crt, err := utils.GenerateSelfSignedSigningCert(entity, nil, time.Hour)
	require.NoError(t, err)
	return key, crt
}

// write deletes the content of the file, and writes a new `content`,
// making sure to fsync afterwards.
func write(t *testing.T, f *os.File, content []byte) {
	require.NoError(t, f.Truncate(0))
	_, err := f.Seek(0, 0)
	require.NoError(t, err)
	_, err = f.Write(content)
	require.NoError(t, err)
	require.NoError(t, f.Sync())
}
