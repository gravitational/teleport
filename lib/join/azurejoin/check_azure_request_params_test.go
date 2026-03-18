/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package azurejoin

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/join/provision"
)

func TestCheckAzureRequestParamsCheckAndSetDefaults(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	nopGetSubscriptionClient := func(context.Context, string) (azure.SubscriptionClient, error) { return nil, nil }
	tests := []struct {
		name          string
		params        CheckAzureRequestParams
		setup         func(*testing.T)
		errorContains string
	}{
		{
			name: "sets defaults",
			params: CheckAzureRequestParams{
				AzureJoinConfig: &AzureJoinConfig{GetSubscriptionClient: nopGetSubscriptionClient},
				Token:           fakeProvisionToken{},
				Challenge:       "challenge",
				AttestedData:    []byte("attested-data"),
				AccessToken:     "access-token",
				Logger:          logger,
			},
		},
		{
			name: "missing join config",
			params: CheckAzureRequestParams{
				Token:        fakeProvisionToken{},
				Challenge:    "challenge",
				AttestedData: []byte("attested-data"),
				AccessToken:  "access-token",
				Logger:       logger,
			},
			errorContains: "AzureJoinConfig is required",
		},
		{
			name: "missing token",
			params: CheckAzureRequestParams{
				AzureJoinConfig: &AzureJoinConfig{GetSubscriptionClient: nopGetSubscriptionClient},
				Challenge:       "challenge",
				AttestedData:    []byte("attested-data"),
				AccessToken:     "access-token",
				Logger:          logger,
			},
			errorContains: "Token is required",
		},
		{
			name: "missing challenge",
			params: CheckAzureRequestParams{
				AzureJoinConfig: &AzureJoinConfig{GetSubscriptionClient: nopGetSubscriptionClient},
				Token:           fakeProvisionToken{},
				AttestedData:    []byte("attested-data"),
				AccessToken:     "access-token",
				Logger:          logger,
			},
			errorContains: "Challenge is required",
		},
		{
			name: "missing attested data",
			params: CheckAzureRequestParams{
				AzureJoinConfig: &AzureJoinConfig{GetSubscriptionClient: nopGetSubscriptionClient},
				Token:           fakeProvisionToken{},
				Challenge:       "challenge",
				AccessToken:     "access-token",
				Logger:          logger,
			},
			errorContains: "AttestedData is required",
		},
		{
			name: "missing access token",
			params: CheckAzureRequestParams{
				AzureJoinConfig: &AzureJoinConfig{GetSubscriptionClient: nopGetSubscriptionClient},
				Token:           fakeProvisionToken{},
				Challenge:       "challenge",
				AttestedData:    []byte("attested-data"),
				Logger:          logger,
			},
			errorContains: "AccessToken is required",
		},
		{
			name: "missing logger",
			params: CheckAzureRequestParams{
				AzureJoinConfig: &AzureJoinConfig{GetSubscriptionClient: nopGetSubscriptionClient},
				Token:           fakeProvisionToken{},
				Challenge:       "challenge",
				AttestedData:    []byte("attested-data"),
				AccessToken:     "access-token",
			},
			errorContains: "Logger is required",
		},
		{
			name: "returns config validation error",
			setup: func(t *testing.T) {
				original := rawAzureCerts
				rawAzureCerts = [][]byte{[]byte("-----BEGIN CERTIFICATE-----\nZm9v\n-----END CERTIFICATE-----\n")}
				t.Cleanup(func() {
					rawAzureCerts = original
				})
			},
			params: CheckAzureRequestParams{
				AzureJoinConfig: &AzureJoinConfig{GetSubscriptionClient: nopGetSubscriptionClient},
				Token:           fakeProvisionToken{},
				Challenge:       "challenge",
				AttestedData:    []byte("attested-data"),
				AccessToken:     "access-token",
				Logger:          logger,
			},
			errorContains: "x509",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}

			params := tc.params
			err := params.checkAndSetDefaults()
			if tc.errorContains != "" {
				require.ErrorContains(t, err, tc.errorContains)
				return
			}
			require.NoError(t, err)
			// ensure default params are set
			require.NotNil(t, params.Clock)
			require.NotNil(t, params.AzureJoinConfig)
			require.NotNil(t, params.AzureJoinConfig.Verify)
			require.NotEmpty(t, params.AzureJoinConfig.CertificateAuthorities)
			require.NotNil(t, params.AzureJoinConfig.GetVMClient)
			require.NotNil(t, params.AzureJoinConfig.IssuerHTTPClient)
		})
	}
}

type fakeProvisionToken struct {
	provision.Token
}
