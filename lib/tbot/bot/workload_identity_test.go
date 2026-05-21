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

package bot

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrustDomainsSelector_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		selector    TrustDomainsSelector
		expectedRes require.ValueAssertionFunc
		expectedErr require.ErrorAssertionFunc
	}{
		"nil selector": {
			selector:    nil,
			expectedRes: require.Nil,
			expectedErr: require.NoError,
		},
		"empty selector": {
			selector:    TrustDomainsSelector{},
			expectedRes: require.Empty,
			expectedErr: require.NoError,
		},
		"valid app_client": {
			selector: TrustDomainsSelector{TrustDomainAppClient},
			expectedRes: func(tt require.TestingT, i1 any, i2 ...any) {
				require.ElementsMatch(t, TrustDomainsSelector{TrustDomainAppClient}, i1, i2...)
			},
			expectedErr: require.NoError,
		},
		"deduplicates entries": {
			selector: TrustDomainsSelector{TrustDomainAppClient, TrustDomainAppClient},
			expectedRes: func(tt require.TestingT, i1 any, i2 ...any) {
				require.ElementsMatch(t, TrustDomainsSelector{TrustDomainAppClient}, i1, i2...)
			},
			expectedErr: require.NoError,
		},
		"unknown trust domain": {
			selector:    TrustDomainsSelector{TrustDomain("random")},
			expectedRes: require.NotNil,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, `invalid trust domain "random". supported trust_domains: "app_client"`, i...)
			},
		},
		"empty trust domain entry": {
			selector:    TrustDomainsSelector{TrustDomain("")},
			expectedRes: require.NotNil,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, `invalid trust domain "". supported trust_domains: "app_client"`, i...)
			},
		},
		"valid entry followed by invalid entry": {
			selector:    TrustDomainsSelector{TrustDomainAppClient, TrustDomain("random")},
			expectedRes: require.NotNil,
			expectedErr: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, `invalid trust domain "random"`, i...)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.selector.CheckAndSetDefaults()
			tc.expectedErr(t, err)
			tc.expectedRes(t, tc.selector)
		})
	}
}
