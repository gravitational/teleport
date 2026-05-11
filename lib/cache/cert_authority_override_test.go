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

package cache

import (
	"context"
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestCertAuthorityOverrides(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	const caType = types.DatabaseClientCA

	testResources153(t, p, testFuncs[*subcav1.CertAuthorityOverride]{
		newResource: func(name string) (*subcav1.CertAuthorityOverride, error) {
			return newCertAuthorityOverride(caType, name), nil
		},
		create: func(ctx context.Context, r *subcav1.CertAuthorityOverride) error {
			_, err := p.subCA.CreateCertAuthorityOverride(ctx, r)
			return err
		},
		list: func(ctx context.Context, pageSize int, pageToken string) ([]*subcav1.CertAuthorityOverride, string, error) {
			return p.subCA.ListCertAuthorityOverrides(ctx, pageSize, pageToken)
		},
		cacheGet: func(ctx context.Context, name string) (*subcav1.CertAuthorityOverride, error) {
			return p.cache.GetCertAuthorityOverride(ctx, types.CertAuthorityOverrideID{
				CAType:      string(caType),
				ClusterName: name,
			})
		},
		cacheList: p.cache.ListCertAuthorityOverrides,
		update: func(ctx context.Context, r *subcav1.CertAuthorityOverride) error {
			_, err := p.subCA.UpdateCertAuthorityOverride(ctx, r)
			return err
		},
		delete: func(ctx context.Context, name string) error {
			return p.subCA.DeleteCertAuthorityOverride(ctx, types.CertAuthorityOverrideID{
				CAType:      string(caType),
				ClusterName: name,
			})
		},
		deleteAll: func(ctx context.Context) error {
			const pageSize = 1000 // Arbitrary.
			pageToken := ""
			for {
				resources, nextPageToken, err := p.subCA.ListCertAuthorityOverrides(ctx, pageSize, pageToken)
				if err != nil {
					return fmt.Errorf("deleteAll: list overrides: %w", err)
				}
				for _, r := range resources {
					if err := p.subCA.DeleteCertAuthorityOverride(ctx, local.CertAuthorityOverrideIDFromResource(r)); err != nil {
						return fmt.Errorf("deleteAll: delete resource %s/%s", r.GetSubKind(), r.GetMetadata().GetName())
					}
				}
				if nextPageToken == "" {
					break
				}
				pageToken = nextPageToken
			}
			return nil
		},
	})
}

func TestCertAuthorityOverrides_notImplemented(t *testing.T) {
	t.Parallel()

	service := notImplementedSubCAService{}
	p := newTestPack(t, func(c Config) Config {
		c = ForAuth(c)
		c.SubCAService = service
		c.neverOK = true // Force upstream reads.
		return c
	})
	t.Cleanup(p.Close)

	cache := p.cache
	clusterName := p.clusterConfigS.GetName()

	t.Run("Get", func(t *testing.T) {
		t.Parallel()

		_, err := cache.GetCertAuthorityOverride(t.Context(), types.CertAuthorityOverrideID{
			ClusterName: clusterName,
			CAType:      string(types.WindowsCA),
		})
		assert.ErrorAs(t, err, new(*trace.NotFoundError))
		assert.ErrorContains(t, err, "ca overrides not implemented")
	})

	t.Run("List", func(t *testing.T) {
		t.Parallel()

		const pageSize = 0
		const pageToken = ""
		got, nextPageToken, err := cache.ListCertAuthorityOverrides(t.Context(), pageSize, pageToken)
		require.NoError(t, err)
		assert.Empty(t, got, "got unexpected results")
		assert.Empty(t, nextPageToken, "got unexpected nextPageToken")
	})
}

func newCertAuthorityOverride(caType types.CertAuthType, clusterName string) *subcav1.CertAuthorityOverride {
	return &subcav1.CertAuthorityOverride{
		Kind:    types.KindCertAuthorityOverride,
		Version: types.V1,
		SubKind: string(caType),
		Metadata: &headerv1.Metadata{
			Name: clusterName,
		},
		Spec: &subcav1.CertAuthorityOverrideSpec{},
	}
}

type notImplementedSubCAService struct{}

func (notImplementedSubCAService) GetCertAuthorityOverride(
	ctx context.Context, id types.CertAuthorityOverrideID) (*subcav1.CertAuthorityOverride, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (notImplementedSubCAService) ListCertAuthorityOverrides(
	ctx context.Context,
	pageSize int,
	pageToken string,
) (_ []*subcav1.CertAuthorityOverride, nextPageToken string, _ error) {
	return nil, "", trace.NotImplemented("not implemented")
}
