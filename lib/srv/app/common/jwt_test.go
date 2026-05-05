/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package common

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestRolesAndTraitsForAppToken(t *testing.T) {
	identity := &tlsca.Identity{
		Username: "test",
		Groups:   []string{"access", "editor"},
		Traits: wrappers.Traits{
			"team": []string{"dev"},
		},
	}

	tests := []struct {
		name         string
		inputRewrite *types.Rewrite
		wantRoles    []string
		wantTraits   wrappers.Traits
	}{
		{
			name:       "nil rewrite",
			wantRoles:  identity.Groups,
			wantTraits: identity.Traits,
		},
		{
			name:         "empty JWTClaims",
			inputRewrite: &types.Rewrite{},
			wantRoles:    identity.Groups,
			wantTraits:   identity.Traits,
		},
		{
			name: "roles only",
			inputRewrite: &types.Rewrite{
				JWTClaims: types.JWTClaimsRewriteRoles,
			},
			wantRoles: identity.Groups,
		},
		{
			name: "traits only",
			inputRewrite: &types.Rewrite{
				JWTClaims: types.JWTClaimsRewriteTraits,
			},
			wantTraits: identity.Traits,
		},
		{
			name: "none",
			inputRewrite: &types.Rewrite{
				JWTClaims: types.JWTClaimsRewriteNone,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := types.NewAppV3(
				types.Metadata{Name: t.Name()},
				types.AppSpecV3{
					URI:     "http://localhost:12345",
					Rewrite: tt.inputRewrite,
				},
			)
			require.NoError(t, err)
			actualRoles, actualTraits := RolesAndTraitsForAppToken(identity, app)
			assert.Equal(t, tt.wantRoles, actualRoles)
			assert.Equal(t, tt.wantTraits, actualTraits)
		})
	}
}

type fakeTokenGenerator struct{}

func (f fakeTokenGenerator) GenerateAppToken(_ context.Context, _ types.GenerateAppTokenRequest) (string, error) {
	return "fake-jwt-token", nil
}

func TestGenerateJWTAndTraitsDoesNotMutateIdentity(t *testing.T) {
	identity := &tlsca.Identity{
		Username: "test",
		Groups:   []string{"access", "editor"},
		Traits: wrappers.Traits{
			"team": []string{"dev"},
		},
	}
	originalTraits := maps.Clone(identity.Traits)

	app, err := types.NewAppV3(
		types.Metadata{Name: "test-app"},
		types.AppSpecV3{URI: "http://localhost:12345"},
	)
	require.NoError(t, err)

	jwt, rewriteTraits, err := GenerateJWTAndTraits(
		t.Context(),
		identity,
		app,
		&fakeTokenGenerator{},
		time.Now().Add(time.Hour).In(time.UTC),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, jwt)

	assert.Equal(t, []string{"fake-jwt-token"}, rewriteTraits[constants.TraitJWT])

	assert.Equal(t, originalTraits, identity.Traits)
	assert.Empty(t, identity.Traits[constants.TraitJWT], "identity.Traits must not contain the JWT")
}
