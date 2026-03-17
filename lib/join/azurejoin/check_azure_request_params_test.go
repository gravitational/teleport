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
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/join/provision"
)

func TestCheckAzureRequestParamsCheckAndSetDefaults(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := []struct {
		name        string
		params      CheckAzureRequestParams
		setup       func(*testing.T)
		assertError require.ErrorAssertionFunc
		errorText   string
		assert      func(*testing.T, *CheckAzureRequestParams)
	}{
		{
			name: "sets defaults when config and clock are nil",
			params: CheckAzureRequestParams{
				Token:        fakeProvisionToken{},
				Challenge:    "challenge",
				AttestedData: []byte("attested-data"),
				AccessToken:  "access-token",
				Logger:       logger,
			},
			assertError: require.NoError,
			assert: func(t *testing.T, params *CheckAzureRequestParams) {
				require.NotNil(t, params.AzureJoinConfig)
				require.NotNil(t, params.Clock)
				require.NotNil(t, params.AzureJoinConfig.Verify)
				require.NotEmpty(t, params.AzureJoinConfig.CertificateAuthorities)
				require.NotNil(t, params.AzureJoinConfig.GetVMClient)
				require.NotNil(t, params.AzureJoinConfig.IssuerHTTPClient)
			},
		},
		{
			name: "missing token",
			params: CheckAzureRequestParams{
				Challenge:    "challenge",
				AttestedData: []byte("attested-data"),
				AccessToken:  "access-token",
				Logger:       logger,
			},
			assertError: require.Error,
			errorText:   "Token is required",
			assert:      func(*testing.T, *CheckAzureRequestParams) {},
		},
		{
			name: "missing challenge",
			params: CheckAzureRequestParams{
				Token:        fakeProvisionToken{},
				AttestedData: []byte("attested-data"),
				AccessToken:  "access-token",
				Logger:       logger,
			},
			assertError: require.Error,
			errorText:   "Challenge is required",
			assert:      func(*testing.T, *CheckAzureRequestParams) {},
		},
		{
			name: "missing attested data",
			params: CheckAzureRequestParams{
				Token:       fakeProvisionToken{},
				Challenge:   "challenge",
				AccessToken: "access-token",
				Logger:      logger,
			},
			assertError: require.Error,
			errorText:   "AttestedData is required",
			assert:      func(*testing.T, *CheckAzureRequestParams) {},
		},
		{
			name: "missing access token",
			params: CheckAzureRequestParams{
				Token:        fakeProvisionToken{},
				Challenge:    "challenge",
				AttestedData: []byte("attested-data"),
				Logger:       logger,
			},
			assertError: require.Error,
			errorText:   "AccessToken is required",
			assert:      func(*testing.T, *CheckAzureRequestParams) {},
		},
		{
			name: "missing logger",
			params: CheckAzureRequestParams{
				Token:        fakeProvisionToken{},
				Challenge:    "challenge",
				AttestedData: []byte("attested-data"),
				AccessToken:  "access-token",
			},
			assertError: require.Error,
			errorText:   "Logger is required",
			assert:      func(*testing.T, *CheckAzureRequestParams) {},
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
				Token:        fakeProvisionToken{},
				Challenge:    "challenge",
				AttestedData: []byte("attested-data"),
				AccessToken:  "access-token",
				Logger:       logger,
			},
			assertError: require.Error,
			errorText:   "x509",
			assert:      func(*testing.T, *CheckAzureRequestParams) {},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}

			params := tc.params
			err := params.checkAndSetDefaults()
			tc.assertError(t, err)
			if tc.errorText != "" {
				require.ErrorContains(t, err, tc.errorText)
			}
			tc.assert(t, &params)
		})
	}
}

type fakeProvisionToken struct {
	provision.Token
}
