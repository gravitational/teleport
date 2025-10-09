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

package samlidpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSAMLIdPBuildScriptCheckAndSetDefaults(t *testing.T) {
	tests := []struct {
		name               string
		organizationID     string
		poolName           string
		poolProviderName   string
		samlIdPMetadataURL string
		errAssertion       require.ErrorAssertionFunc
	}{
		{
			name:           "empty organization id",
			organizationID: "",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "required")
			},
		},
		{
			name:               "organization id with alphabet",
			organizationID:     "123abc123",
			poolName:           "test-pool-name",
			poolProviderName:   "test-pool-provider-name",
			samlIdPMetadataURL: "https://metadata",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "numeric value")
			},
		},
		{
			name:               "valid organization name",
			organizationID:     "123423452",
			poolName:           "test-pool-name",
			poolProviderName:   "test-pool-provider-name",
			samlIdPMetadataURL: "https://metadata",
			errAssertion:       require.NoError,
		},
		{
			name:           "empty pool name",
			organizationID: "123423452",
			poolName:       "",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "required")
			},
		},
		{
			name:             "empty pool provider name",
			organizationID:   "123423452",
			poolName:         "test-pool-name",
			poolProviderName: "",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "required")
			},
		},
		{
			name:               "empty idpMetadataURL",
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
			newParams := &GCPWorkforceAPIParams{
				OrganizationID:     test.organizationID,
				PoolName:           test.poolName,
				PoolProviderName:   test.poolProviderName,
				SAMLIdPMetadataURL: test.samlIdPMetadataURL,
			}
			err := newParams.CheckAndSetDefaults()
			test.errAssertion(t, err)
		})
	}
}

func TestValidateGCPResourceName(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "empty name",
			value: "",
		},
		{
			name:  "longer than 63 character",
			value: "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij",
		},
		{
			name:  "starts with number",
			value: "1abcde",
		},
		{
			name:  "contains underscore",
			value: "abcde_abcde",
		},
		{
			name:  "contains uppercase character",
			value: "abcdeABCDEabcde",
		},
		{
			name:  "contains hyphen at the end",
			value: "abcde-",
		},
		{
			name:  "contains asterisk at the end",
			value: "abc*de",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateGCPResourceName(test.value, "noop")
			require.Error(t, err)
		})
	}
}
