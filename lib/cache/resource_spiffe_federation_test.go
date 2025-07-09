// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package cache

import (
	"context"
	"testing"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
)

func newSPIFFEFederation(name string) *machineidv1.SPIFFEFederation {
	return &machineidv1.SPIFFEFederation{
		Kind:    types.KindSPIFFEFederation,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &machineidv1.SPIFFEFederationSpec{
			BundleSource: &machineidv1.SPIFFEFederationBundleSource{
				HttpsWeb: &machineidv1.SPIFFEFederationBundleSourceHTTPSWeb{
					BundleEndpointUrl: "https://example.com/bundle.json",
				},
			},
		},
	}
}

func TestSPIFFEFederations(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*machineidv1.SPIFFEFederation]{
		newResource: func(s string) (*machineidv1.SPIFFEFederation, error) {
			return newSPIFFEFederation(s), nil
		},

		create: func(ctx context.Context, item *machineidv1.SPIFFEFederation) error {
			_, err := p.spiffeFederations.CreateSPIFFEFederation(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*machineidv1.SPIFFEFederation, error) {
			items, _, err := p.spiffeFederations.ListSPIFFEFederations(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		deleteAll: func(ctx context.Context) error {
			return p.spiffeFederations.DeleteAllSPIFFEFederations(ctx)
		},

		cacheList: func(ctx context.Context) ([]*machineidv1.SPIFFEFederation, error) {
			items, _, err := p.cache.ListSPIFFEFederations(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		cacheGet: p.cache.GetSPIFFEFederation,
	})
}
