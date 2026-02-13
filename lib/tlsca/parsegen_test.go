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

	"github.com/stretchr/testify/assert"

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
