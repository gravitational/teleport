// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestValidatAppAuthConfig(t *testing.T) {
	t.Parallel()

	t.Run("Base", func(t *testing.T) {
		validConfig := &appauthconfigv1.AppAuthConfigSpec_Jwt{
			Jwt: &appauthconfigv1.AppAuthConfigJWTSpec{
				Audience: "teleport",
				Issuer:   "https://issuer-url/",
				KeysSource: &appauthconfigv1.AppAuthConfigJWTSpec_JwksUrl{
					JwksUrl: "https://issuer-url/.well-known/jwks.json",
				},
			},
		}
		require.NoError(t, validateJWTAppAuthConfig(validConfig.Jwt), "this test expects the config to be valid, ensure it is up-to-date")

		for name, tc := range map[string]struct {
			res       *appauthconfigv1.AppAuthConfig
			assertErr require.ErrorAssertionFunc
		}{
			"default is valid": {
				res: &appauthconfigv1.AppAuthConfig{
					Version: types.V1,
					Kind:    types.KindAppAuthConfig,
					Metadata: &headerv1.Metadata{
						Name: "example",
					},
					Spec: &appauthconfigv1.AppAuthConfigSpec{
						AppLabels: []*labelv1.Label{
							{Name: "*", Values: []string{"*"}},
						},
						SubKindSpec: validConfig,
					},
				},
				assertErr: require.NoError,
			},
			"unknown is invalid": {
				res: &appauthconfigv1.AppAuthConfig{
					Version: "999",
					Kind:    types.KindAppAuthConfig,
					Metadata: &headerv1.Metadata{
						Name: "example",
					},
					Spec: &appauthconfigv1.AppAuthConfigSpec{
						AppLabels: []*labelv1.Label{
							{Name: "*", Values: []string{"*"}},
						},
						SubKindSpec: validConfig,
					},
				},
				assertErr: require.Error,
			},
			"missing metadata is invalid": {
				res: &appauthconfigv1.AppAuthConfig{
					Version: types.V1,
					Kind:    types.KindAppAuthConfig,
					Spec: &appauthconfigv1.AppAuthConfigSpec{
						AppLabels: []*labelv1.Label{
							{Name: "*", Values: []string{"*"}},
						},
						SubKindSpec: validConfig,
					},
				},
				assertErr: require.Error,
			},
			"missing app labels is invalid": {
				res: &appauthconfigv1.AppAuthConfig{
					Version: types.V1,
					Kind:    types.KindAppAuthConfig,
					Metadata: &headerv1.Metadata{
						Name: "example",
					},
					Spec: &appauthconfigv1.AppAuthConfigSpec{
						SubKindSpec: validConfig,
					},
				},
				assertErr: require.Error,
			},
			"invalid app labels wildcard is invalid": {
				res: &appauthconfigv1.AppAuthConfig{
					Version: types.V1,
					Kind:    types.KindAppAuthConfig,
					Metadata: &headerv1.Metadata{
						Name: "example",
					},
					Spec: &appauthconfigv1.AppAuthConfigSpec{
						AppLabels: []*labelv1.Label{
							{Name: "*", Values: []string{"some-random-value"}},
						},
						SubKindSpec: validConfig,
					},
				},
				assertErr: require.Error,
			},
			"nil is invalid": {
				res:       nil,
				assertErr: require.Error,
			},
		} {
			t.Run(name, func(t *testing.T) {
				tc.assertErr(t, ValidateAppAuthConfig(tc.res))
			})
		}
	})

	t.Run("JWT config", func(t *testing.T) {
		for name, tc := range map[string]struct {
			res       *appauthconfigv1.AppAuthConfigJWTSpec
			assertErr require.ErrorAssertionFunc
		}{
			"minimal is valid": {
				res: &appauthconfigv1.AppAuthConfigJWTSpec{
					Audience: "teleport",
					Issuer:   "https://issuer-url/",
					KeysSource: &appauthconfigv1.AppAuthConfigJWTSpec_JwksUrl{
						JwksUrl: "https://issuer-url/.well-known/jwks.json",
					},
				},
				assertErr: require.NoError,
			},
			"missing jwks_url and static_jwks is invalid": {
				res: &appauthconfigv1.AppAuthConfigJWTSpec{
					Audience: "teleport",
				},
				assertErr: require.Error,
			},
			"missing audience is invalid": {
				res: &appauthconfigv1.AppAuthConfigJWTSpec{
					Issuer: "https://issuer-url/",
					KeysSource: &appauthconfigv1.AppAuthConfigJWTSpec_JwksUrl{
						JwksUrl: "https://issuer-url/.well-known/jwks.json",
					},
				},
				assertErr: require.Error,
			},
			"nil is invalid": {
				res:       nil,
				assertErr: require.Error,
			},
		} {
			t.Run(name, func(t *testing.T) {
				tc.assertErr(t, validateJWTAppAuthConfig(tc.res))
			})
		}
	})
}
