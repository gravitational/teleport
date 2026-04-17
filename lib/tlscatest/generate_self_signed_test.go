// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package tlscatest_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/tlscatest"
)

func TestGenerateSelfSignedCA(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		const clusterName = "localhost"
		keyPEM, certPEM, err := tlscatest.GenerateSelfSignedCA(tlscatest.GenerateCAConfig{
			ClusterName: clusterName,
		})
		require.NoError(t, err)

		// Parse key.
		_, err = keys.ParsePrivateKey(keyPEM)
		require.NoError(t, err, "Parse private key")

		// Parse cert.
		cert, err := tlsca.ParseCertificatePEM(certPEM)
		require.NoError(t, err, "Parse certificate")

		// Verify timestamps.
		now := time.Now().Add(1 * time.Nanosecond) // add 1ns just to extra safe.
		assert.True(t, cert.NotBefore.Before(now), "NotBefore in the future")
		assert.True(t, cert.NotAfter.After(now), "NotAfter in the past")

		// Verify cluster name in certificate.
		gotClusterName, err := tlsca.ClusterName(cert.Subject)
		require.NoError(t, err, "Extract cluster name from certificate")
		assert.Equal(t, clusterName, gotClusterName, "Cluster name mismatch")
	})

	t.Run("custom timestamps", func(t *testing.T) {
		t.Parallel()

		// Timestamps need to be UTC and at second precision.
		notBefore := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
		notAfter := time.Date(2126, 2, 2, 0, 0, 0, 0, time.UTC)

		_, certPEM, err := tlscatest.GenerateSelfSignedCA(tlscatest.GenerateCAConfig{
			ClusterName: "localhost",
			NotBefore:   notBefore,
			NotAfter:    notAfter,
		})
		require.NoError(t, err)

		cert, err := tlsca.ParseCertificatePEM(certPEM)
		require.NoError(t, err)
		assert.Equal(t, notBefore, cert.NotBefore, "NotBefore mismatch")
		assert.Equal(t, notAfter, cert.NotAfter, "NotAfter mismatch")
	})
}
