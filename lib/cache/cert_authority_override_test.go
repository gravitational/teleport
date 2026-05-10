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
