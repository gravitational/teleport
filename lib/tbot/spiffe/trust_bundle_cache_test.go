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

package spiffe

import (
	"testing"

	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/require"
)

func TestBundleSet_Clone(t *testing.T) {
	t.Parallel()

	// We don't need to test too thoroughly since most of the Clone
	// implementation comes from spiffe-go. We just want to ensure both the
	// local and federated bundles are cloned.

	localTD := spiffeid.RequireTrustDomainFromString("example.com")
	localBundle := spiffebundle.New(localTD)

	federatedTD := spiffeid.RequireTrustDomainFromString("federated.example.com")
	federatedBundle := spiffebundle.New(federatedTD)

	bundleSet := BundleSet{
		Local: localBundle,
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com": federatedBundle,
		},
	}

	clonedBundleSet := bundleSet.Clone()
	require.True(t, bundleSet.Equal(clonedBundleSet))

	bundleSet.Local.SetSequenceNumber(12)
	bundleSet.Federated["federated.example.com"].SetSequenceNumber(13)
	require.False(t, bundleSet.Equal(clonedBundleSet))
}

func TestBundleSet_Equal(t *testing.T) {
	t.Parallel()

	bundleSet1 := &BundleSet{
		Local: spiffebundle.New(spiffeid.RequireTrustDomainFromString("example.com")),
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com": spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com")),
		},
	}
	bundleSet2 := &BundleSet{
		Local: spiffebundle.New(spiffeid.RequireTrustDomainFromString("example.com")),
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com":  spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com")),
			"federated2.example.com": spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated2.example.com")),
		},
	}
	bundleSet3 := &BundleSet{
		Local: spiffebundle.New(spiffeid.RequireTrustDomainFromString("2.example.com")),
		Federated: map[string]*spiffebundle.Bundle{
			"federated.example.com": spiffebundle.New(spiffeid.RequireTrustDomainFromString("federated.example.com")),
		},
	}

	// We don't need to test too thoroughly since most of the Equals
	// implementation comes from spiffe-go.
	tests := []struct {
		name  string
		a     *BundleSet
		b     *BundleSet
		equal bool
	}{
		{
			name:  "bundle set 1 equal",
			a:     bundleSet1,
			b:     bundleSet1,
			equal: true,
		},
		{
			name:  "bundle set 2 equal",
			a:     bundleSet2,
			b:     bundleSet2,
			equal: true,
		},
		{
			name:  "bundle set 3 equal",
			a:     bundleSet3,
			b:     bundleSet3,
			equal: true,
		},
		{
			name: "bundle set 1 and 2 not equal",
			a:    bundleSet1,
			b:    bundleSet2,
		},
		{
			name: "bundle set 1 and 3 not equal",
			a:    bundleSet1,
			b:    bundleSet3,
		},
		{
			name: "bundle set 2 and 3 not equal",
			a:    bundleSet2,
			b:    bundleSet3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.equal, tt.a.Equal(tt.b))
		})
	}
}
