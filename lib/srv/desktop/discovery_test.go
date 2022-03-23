// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package desktop

import (
	"strconv"
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

// TestDiscoveryLDAPFilter verifies that WindowsService produces a valid
// LDAP filter when given valid configuration.
func TestDiscoveryLDAPFilter(t *testing.T) {
	for _, test := range []struct {
		desc    string
		filters []string
		assert  require.ErrorAssertionFunc
	}{
		{
			desc:   "OK - no custom filters",
			assert: require.NoError,
		},
		{
			desc:    "OK - custom filters",
			filters: []string{"(computerName=test)", "(location=Oakland)"},
			assert:  require.NoError,
		},
		{
			desc:    "NOK - invalid custom filter",
			filters: []string{"invalid"},
			assert:  require.Error,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			s := &WindowsService{
				cfg: WindowsServiceConfig{
					DiscoveryLDAPFilters: test.filters,
				},
			}

			filter := s.ldapSearchFilter()
			_, err := ldap.CompileFilter(filter)
			test.assert(t, err)
		})
	}
}

func TestAppliesLDAPLabels(t *testing.T) {
	l := make(map[string]string)
	entry := ldap.NewEntry("CN=test,DC=example,DC=com", map[string][]string{
		attrDNSHostName: {"foo.example.com"},
		attrName:        {"foo"},
		attrOS:          {"Windows Server"},
		attrOSVersion:   {"6.1"},
	})
	applyLabelsFromLDAP(entry, l)

	require.Equal(t, l[types.OriginLabel], types.OriginDynamic)
	require.Equal(t, l["teleport.dev/dns_host_name"], "foo.example.com")
	require.Equal(t, l["teleport.dev/computer_name"], "foo")
	require.Equal(t, l["teleport.dev/os"], "Windows Server")
	require.Equal(t, l["teleport.dev/os_version"], "6.1")
}

func TestLabelsDomainControllers(t *testing.T) {
	for _, test := range []struct {
		desc   string
		entry  *ldap.Entry
		assert require.BoolAssertionFunc
	}{
		{
			desc: "DC",
			entry: ldap.NewEntry("CN=test,DC=example,DC=com", map[string][]string{
				attrPrimaryGroupID: {writableDomainControllerGroupID},
			}),
			assert: require.True,
		},
		{
			desc: "RODC",
			entry: ldap.NewEntry("CN=test,DC=example,DC=com", map[string][]string{
				attrPrimaryGroupID: {readOnlyDomainControllerGroupID},
			}),
			assert: require.True,
		},
		{
			desc: "computer",
			entry: ldap.NewEntry("CN=test,DC=example,DC=com", map[string][]string{
				attrPrimaryGroupID: {"515"},
			}),
			assert: require.False,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			l := make(map[string]string)
			applyLabelsFromLDAP(test.entry, l)

			b, _ := strconv.ParseBool(l["teleport.dev/is_domain_controller"])
			test.assert(t, b)
		})
	}
}
