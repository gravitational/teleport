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

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
)

func TestParseJoinURI(t *testing.T) {
	tests := []struct {
		uri         string
		expect      *JoinURIParams
		expectError require.ErrorAssertionFunc
	}{
		{
			uri: "tbot+proxy+token://asdf@example.com:1234",
			expect: &JoinURIParams{
				AddressKind:         connection.AddressKindProxy,
				Token:               "asdf",
				JoinMethod:          types.JoinMethodToken,
				Address:             "example.com:1234",
				JoinMethodParameter: "",
			},
		},
		{
			uri: "tbot+auth+bound-keypair://token:param@example.com",
			expect: &JoinURIParams{
				AddressKind:         connection.AddressKindAuth,
				Token:               "token",
				JoinMethod:          types.JoinMethodBoundKeypair,
				Address:             "example.com",
				JoinMethodParameter: "param",
			},
		},
		{
			uri: "",
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "unsupported joining URI scheme")
			},
		},
		{
			uri: "tbot+foo+token://example.com",
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "address kind must be one of")
			},
		},
		{
			uri: "tbot+proxy+bar://example.com",
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "unsupported join method")
			},
		},
		{
			uri: "https://example.com",
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "unsupported joining URI scheme")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			parsed, err := ParseJoinURI(tt.uri)
			if tt.expectError == nil {
				require.NoError(t, err)
			} else {
				tt.expectError(t, err)
			}

			require.Empty(t, cmp.Diff(parsed, tt.expect))
		})
	}
}

func TestJoinURIApplyToConfig(t *testing.T) {
	tests := []struct {
		uri          string
		inputConfig  *BotConfig
		expectConfig *BotConfig
		expectError  require.ErrorAssertionFunc
	}{
		{
			uri:         "tbot+proxy+token://asdf@example.com:1234",
			inputConfig: &BotConfig{},
			expectConfig: &BotConfig{
				Onboarding: onboarding.Config{
					TokenValue: "asdf",
					JoinMethod: types.JoinMethodToken,
				},
				ProxyServer: "example.com:1234",
			},
		},
		{
			uri:         "tbot+proxy+bound-keypair://some-token:secret@example.com:1234",
			inputConfig: &BotConfig{},
			expectConfig: &BotConfig{
				Onboarding: onboarding.Config{
					TokenValue: "some-token",
					JoinMethod: types.JoinMethodBoundKeypair,
					BoundKeypair: onboarding.BoundKeypairOnboardingConfig{
						RegistrationSecretValue: "secret",
					},
				},
				ProxyServer: "example.com:1234",
			},
		},
		{
			uri:         "tbot+auth+azure://some-token:client-id@example.com:1234",
			inputConfig: &BotConfig{},
			expectConfig: &BotConfig{
				Onboarding: onboarding.Config{
					TokenValue: "some-token",
					JoinMethod: types.JoinMethodAzure,
					Azure: onboarding.AzureOnboardingConfig{
						ClientID: "client-id",
					},
				},
				AuthServer: "example.com:1234",
			},
		},
		{
			uri:         "tbot+auth+gitlab://some-token:var-name@example.com:1234",
			inputConfig: &BotConfig{},
			expectConfig: &BotConfig{
				Onboarding: onboarding.Config{
					TokenValue: "some-token",
					JoinMethod: types.JoinMethodGitLab,
					Gitlab: onboarding.GitlabOnboardingConfig{
						TokenEnvVarName: "var-name",
					},
				},
				AuthServer: "example.com:1234",
			},
		},
		{
			uri:         "tbot+auth+azure-devops://some-token@example.com:1234",
			inputConfig: &BotConfig{},
			expectConfig: &BotConfig{
				Onboarding: onboarding.Config{
					TokenValue: "some-token",
					JoinMethod: types.JoinMethodAzureDevops,
				},
				AuthServer: "example.com:1234",
			},
		},
		{
			uri:         "tbot+auth+terraform-cloud://some-token:tag@example.com:1234",
			inputConfig: &BotConfig{},
			expectConfig: &BotConfig{
				Onboarding: onboarding.Config{
					TokenValue: "some-token",
					JoinMethod: types.JoinMethodTerraformCloud,
					Terraform: onboarding.TerraformOnboardingConfig{
						AudienceTag: "tag",
					},
				},
				AuthServer: "example.com:1234",
			},
		},
		{
			uri: "tbot+proxy+token://asdf@example.com:1234",
			inputConfig: &BotConfig{
				AuthServer: "example.com",
			},
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "URI conflicts with configured field: auth_server")
			},
		},
		{
			uri: "tbot+auth+token://asdf@example.com:1234",
			inputConfig: &BotConfig{
				ProxyServer: "example.com",
			},
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "URI conflicts with configured field: proxy_server")
			},
		},
		{
			uri: "tbot+auth+bound-keypair://asdf:secret@example.com:1234",
			inputConfig: &BotConfig{
				ProxyServer: "example.com",
				Onboarding: onboarding.Config{
					TokenValue: "token",
					JoinMethod: types.JoinMethodBoundKeypair,
					BoundKeypair: onboarding.BoundKeypairOnboardingConfig{
						RegistrationSecretValue: "secret2",
					},
				},
			},
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "field: onboarding.token")
				require.ErrorContains(tt, err, "field: onboarding.bound_keypair.registration_secret")
				require.ErrorContains(tt, err, "field: proxy_server")

				// Note: join method is already bound_keypair so no error will
				// be raised for that field.
				require.NotContains(tt, err.Error(), "field: join_method")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			parsed, err := ParseJoinURI(tt.uri)
			require.NoError(t, err)

			err = parsed.ApplyToConfig(tt.inputConfig)
			if tt.expectError != nil {
				tt.expectError(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectConfig, tt.inputConfig)
			}
		})
	}
}
