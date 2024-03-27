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
		name             string
		orgID            string
		poolName         string
		poolProviderName string
		httpClient       *http.Client
		errAssertion     require.ErrorAssertionFunc
	}{
		{
			name:             "valid organization name",
			orgID:            "123423452",
			poolName:         "test-pool-name",
			poolProviderName: "test-pool-provider-name",
			httpClient: &http.Client{
				Timeout: defaults.HTTPRequestTimeout,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			},
			errAssertion: require.NoError,
		},
		{
			name:             "missing http client",
			orgID:            "123423452",
			poolName:         "test-pool-name",
			poolProviderName: "test-pool-provider-name",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "param HTTPClient required")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewGCPWorkforceService(GCPWorkforceService{
				APIParams: samlidpconfig.GCPWorkforceAPIParams{
					OrganizationID:     test.orgID,
					PoolName:           test.poolName,
					PoolProviderName:   test.poolProviderName,
					SAMLIdPMetadataURL: "http://metadata",
				},
				HTTPClient: test.httpClient,
			})
			test.errAssertion(t, err)
		})
	}
}
