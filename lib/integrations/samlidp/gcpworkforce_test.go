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

package samlidp

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/integrations/samlidp/samlidpconfig"
)

func TestNewGCPWorkforceService(t *testing.T) {
	tests := []struct {
		name               string
		organizationID     string
		poolName           string
		poolProviderName   string
		samlIdPMetadataURL string
		httpClient         *http.Client
		errAssertion       require.ErrorAssertionFunc
	}{
		{
			name:               "valid organization name",
			organizationID:     "123423452",
			poolName:           "test-pool-name",
			poolProviderName:   "test-pool-provider-name",
			samlIdPMetadataURL: "https://metadata",
			httpClient: &http.Client{
				Timeout: defaults.HTTPRequestTimeout,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			},
			errAssertion: require.NoError,
		},
		{
			name:           "missing organization id",
			organizationID: "",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "required")
			},
		},
		{
			name:           "missing pool name",
			organizationID: "123423452",
			poolName:       "",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "required")
			},
		},
		{
			name:             "missing provider name",
			organizationID:   "123423452",
			poolName:         "test-pool-name",
			poolProviderName: "",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "required")
			},
		},
		{
			name:               "missing IdP metadata URL value",
			organizationID:     "123423452",
			poolName:           "test-pool-name",
			poolProviderName:   "test-pool-provider-name",
			samlIdPMetadataURL: "",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "required")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewGCPWorkforceService(GCPWorkforceService{
				APIParams: samlidpconfig.GCPWorkforceAPIParams{
					OrganizationID:     test.organizationID,
					PoolName:           test.poolName,
					PoolProviderName:   test.poolProviderName,
					SAMLIdPMetadataURL: test.samlIdPMetadataURL,
				},
				HTTPClient: test.httpClient,
			})
			test.errAssertion(t, err)
		})
	}
}
