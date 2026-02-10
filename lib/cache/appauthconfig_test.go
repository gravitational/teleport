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

	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
)

func newAppAuthConfigJWT(name string) *appauthconfigv1.AppAuthConfig {
	return &appauthconfigv1.AppAuthConfig{
		Kind:    types.KindAppAuthConfig,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &appauthconfigv1.AppAuthConfigSpec{
			AppLabels: []*labelv1.Label{{Name: "*", Values: []string{"*"}}},
			SubKindSpec: &appauthconfigv1.AppAuthConfigSpec_Jwt{
				Jwt: &appauthconfigv1.AppAuthConfigJWTSpec{
					Issuer:   "https://issuer",
					Audience: "teleport",
					KeysSource: &appauthconfigv1.AppAuthConfigJWTSpec_JwksUrl{
						JwksUrl: "https://issuer/.well-known/jwks.json",
					},
				},
			},
		},
	}
}

func TestAppAuthConfigs(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*appauthconfigv1.AppAuthConfig]{
		newResource: func(s string) (*appauthconfigv1.AppAuthConfig, error) {
			return newAppAuthConfigJWT(s), nil
		},
		create: func(ctx context.Context, item *appauthconfigv1.AppAuthConfig) error {
			_, err := p.appAuthConfigs.CreateAppAuthConfig(ctx, item)
			return trace.Wrap(err)
		},
		list:      p.appAuthConfigs.ListAppAuthConfigs,
		deleteAll: p.appAuthConfigs.DeleteAllAppAuthConfigs,
		cacheList: p.cache.ListAppAuthConfigs,
		cacheGet:  p.cache.GetAppAuthConfig,
	})
}
