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

package tlsca_test

import (
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestClusterName(t *testing.T) {
	t.Parallel()

	const exampleCluster = "zarquon"

	tests := []struct {
		name     string
		pkixName pkix.Name
		wantErr  string // takes precedence over want
		want     string
	}{
		{
			name: "cluster name in O",
			pkixName: pkix.Name{
				Organization: []string{exampleCluster},
			},
			want: exampleCluster,
		},
		{
			name: "cluster name in OID",
			pkixName: pkix.Name{
				// Use Names, instead of ExtraNames, to mimic a parsed certificate.
				Names: []pkix.AttributeTypeAndValue{
					{Type: tlsca.CAClusterNameExtensionOID, Value: exampleCluster},
				},
			},
			want: exampleCluster,
		},
		{
			name: "OID favored over O",
			pkixName: pkix.Name{
				Organization: []string{"badcluster"},
				Names: []pkix.AttributeTypeAndValue{
					{Type: tlsca.CAClusterNameExtensionOID, Value: "goodcluster"},
				},
			},
			want: "goodcluster",
		},
		{
			name:     "no cluster name",
			pkixName: pkix.Name{},
			wantErr:  "missing cluster name",
		},
		{
			name: "empty O cluster name",
			pkixName: pkix.Name{
				Organization: []string{""},
			},
			wantErr: "empty organization",
		},
		{
			name: "empty OID",
			pkixName: pkix.Name{
				Names: []pkix.AttributeTypeAndValue{
					{Type: tlsca.CAClusterNameExtensionOID, Value: ""},
				},
			},
			wantErr: "empty value",
		},
		{
			name: "invalid OID value type",
			pkixName: pkix.Name{
				Names: []pkix.AttributeTypeAndValue{
					{Type: tlsca.CAClusterNameExtensionOID, Value: 0},
				},
			},
			wantErr: "unexpected type",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := tlsca.ClusterName(test.pkixName)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "ClusterName() error mismatch")
				return
			}
			assert.Equal(t, test.want, got, "Cluster name mismatch")
		})
	}
}

func TestGenerateSelfSignedCAForTesting(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		keyPEM, certPEM, err := tlsca.GenerateSelfSignedCAForTesting(tlsca.GenerateTestCAConfig{
			ClusterName: "localhost",
		})
		require.NoError(t, err)

		_, err = keys.ParsePrivateKey(keyPEM)
		require.NoError(t, err, "Parse private key")

		cert, err := tlsca.ParseCertificatePEM(certPEM)
		require.NoError(t, err, "Parse certificate")

		now := time.Now().Add(1 * time.Nanosecond) // add 1ns just to extra safe.
		assert.True(t, cert.NotBefore.Before(now), "NotBefore in the future")
		assert.True(t, cert.NotAfter.After(now), "NotAfter in the past")
	})

	t.Run("custom timestamps", func(t *testing.T) {
		t.Parallel()

		// Timestamps need to be UTC and at second precision.
		notBefore := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
		notAfter := time.Date(2126, 2, 2, 0, 0, 0, 0, time.UTC)

		_, certPEM, err := tlsca.GenerateSelfSignedCAForTesting(tlsca.GenerateTestCAConfig{
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
