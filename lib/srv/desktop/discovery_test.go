/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package desktop

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/windows"
	logutils "github.com/gravitational/teleport/lib/utils/log"
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
		windows.AttrDNSHostName:       {"foo.example.com"},
		windows.AttrName:              {"foo"},
		windows.AttrOS:                {"Windows Server"},
		windows.AttrOSVersion:         {"6.1"},
		windows.AttrDistinguishedName: {"CN=foo,OU=IT,DC=goteleport,DC=com"},
		windows.AttrCommonName:        {"foo"},
		"bar":                         {"baz"},
		"quux":                        {""},
	})

	s := &WindowsService{
		cfg: WindowsServiceConfig{
			DiscoveryLDAPAttributeLabels: []string{"bar"},
		},
	}
	s.applyLabelsFromLDAP(entry, l)

	// check default labels
	require.Equal(t, types.OriginDynamic, l[types.OriginLabel])
	require.Equal(t, "foo.example.com", l[types.DiscoveryLabelWindowsDNSHostName])
	require.Equal(t, "foo", l[types.DiscoveryLabelWindowsComputerName])
	require.Equal(t, "Windows Server", l[types.DiscoveryLabelWindowsOS])
	require.Equal(t, "6.1", l[types.DiscoveryLabelWindowsOSVersion])

	// check OU label
	require.Equal(t, "OU=IT,DC=goteleport,DC=com", l[types.DiscoveryLabelWindowsOU])

	// check custom labels
	require.Equal(t, "baz", l["ldap/bar"])
	require.Empty(t, l["ldap/quux"])
}

func TestLabelsDomainControllers(t *testing.T) {
	s := &WindowsService{}
	for _, test := range []struct {
		desc   string
		entry  *ldap.Entry
		assert require.BoolAssertionFunc
	}{
		{
			desc: "DC",
			entry: ldap.NewEntry("CN=test,DC=example,DC=com", map[string][]string{
				windows.AttrPrimaryGroupID: {windows.WritableDomainControllerGroupID},
			}),
			assert: require.True,
		},
		{
			desc: "RODC",
			entry: ldap.NewEntry("CN=test,DC=example,DC=com", map[string][]string{
				windows.AttrPrimaryGroupID: {windows.ReadOnlyDomainControllerGroupID},
			}),
			assert: require.True,
		},
		{
			desc: "computer",
			entry: ldap.NewEntry("CN=test,DC=example,DC=com", map[string][]string{
				windows.AttrPrimaryGroupID: {"515"},
			}),
			assert: require.False,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			l := make(map[string]string)
			s.applyLabelsFromLDAP(test.entry, l)

			b, _ := strconv.ParseBool(l[types.DiscoveryLabelWindowsIsDomainController])
			test.assert(t, b)
		})
	}
}

// TestDNSErrors verifies that errors are handled quickly
// and do not block discovery for too long.
func TestDNSErrors(t *testing.T) {
	s := &WindowsService{
		cfg: WindowsServiceConfig{
			Logger: slog.New(logutils.NewSlogTextHandler(io.Discard, logutils.SlogTextHandlerConfig{})),
			Clock:  clockwork.NewRealClock(),
		},
		dnsResolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return nil, errors.New("this resolver always fails")
			},
		},
	}

	start := time.Now()
	_, err := s.lookupDesktop(context.Background(), "$invalid hostname")
	require.Less(t, time.Since(start), dnsQueryTimeout-1*time.Second)
	require.Error(t, err)
}
