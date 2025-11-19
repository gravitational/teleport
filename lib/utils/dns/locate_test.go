// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package dns

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocateServerBySRV(t *testing.T) {
	for _, test := range []struct {
		name           string
		domain         string
		site           string
		service        string
		port           string
		mockSRVRecords map[string][]*net.SRV
		mockSRVErrors  map[string]error
		expectedResult []string
		expectedError  string
	}{
		{
			name:    "successful lookup without site",
			domain:  "example.com",
			site:    "",
			service: "ldap",
			port:    "",
			mockSRVRecords: map[string][]*net.SRV{
				"_ldap._tcp.example.com": {
					{Target: "dc1.example.com.", Port: 389, Priority: 0, Weight: 5},
					{Target: "dc2.example.com.", Port: 389, Priority: 1, Weight: 5},
				},
			},
			expectedResult: []string{"dc1.example.com.:389", "dc2.example.com.:389"},
		},
		{
			name:    "successful lookup with site",
			domain:  "example.com",
			site:    "site1",
			service: "ldap",
			port:    "",
			mockSRVRecords: map[string][]*net.SRV{
				"_ldap._tcp.site1._sites.example.com": {
					{Target: "dc1-site1.example.com.", Port: 389, Priority: 0, Weight: 5},
				},
			},
			expectedResult: []string{"dc1-site1.example.com.:389"},
		},
		{
			name:    "site lookup fails, fallback to domain succeeds",
			domain:  "example.com",
			site:    "site1",
			service: "ldap",
			port:    "",
			mockSRVRecords: map[string][]*net.SRV{
				"_ldap._tcp.example.com": {
					{Target: "dc1.example.com.", Port: 389, Priority: 0, Weight: 5},
				},
			},
			mockSRVErrors: map[string]error{
				"_ldap._tcp.site1._sites.example.com": &net.DNSError{Name: "site1._sites.example.com", Err: "no such host"},
			},
			expectedResult: []string{"dc1.example.com.:389"},
		},
		{
			name:    "both site and domain lookups fail",
			domain:  "example.com",
			site:    "site1",
			service: "ldap",
			port:    "",
			mockSRVErrors: map[string]error{
				"_ldap._tcp.site1._sites.example.com": &net.DNSError{Name: "site1._sites.example.com", Err: "no such host"},
				"_ldap._tcp.example.com":              &net.DNSError{Name: "example.com", Err: "no such host"},
			},
			expectedError: "looking up SRV records",
		},
		{
			name:    "successful port override",
			domain:  "example.com",
			site:    "",
			service: "ldap",
			port:    "636",
			mockSRVRecords: map[string][]*net.SRV{
				"_ldap._tcp.example.com": {
					{Target: "dc1.example.com.", Port: 389, Priority: 0, Weight: 5},
				},
			},
			expectedResult: []string{"dc1.example.com.:636"},
		},
		{
			name:    "empty results",
			domain:  "example.com",
			site:    "",
			service: "ldap",
			port:    "",
			mockSRVRecords: map[string][]*net.SRV{
				"_ldap._tcp.example.com": {},
			},
			expectedResult: nil,
		},
		{
			name:    "multiple records with IPv6 target",
			domain:  "example.com",
			site:    "",
			service: "ldap",
			port:    "",
			mockSRVRecords: map[string][]*net.SRV{
				"_ldap._tcp.example.com": {
					{Target: "dc1.example.com.", Port: 389, Priority: 0, Weight: 5},
					{Target: "2001:db8::1.", Port: 389, Priority: 0, Weight: 5},
				},
			},
			expectedResult: []string{"dc1.example.com.:389", "[2001:db8::1.]:389"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolver := &mockResolver{
				srvRecords: test.mockSRVRecords,
				srvErrors:  test.mockSRVErrors,
			}

			result, err := LocateServerBySRV(context.Background(), test.domain, test.site, resolver, test.service, test.port)

			if test.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.expectedError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expectedResult, result)
		})
	}
}

type mockResolver struct {
	srvRecords map[string][]*net.SRV
	srvErrors  map[string]error
}

func (m *mockResolver) LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error) {
	key := "_" + service + "._" + proto + "." + name

	if err, exists := m.srvErrors[key]; exists {
		return "", nil, err
	}

	if records, exists := m.srvRecords[key]; exists {
		return name, records, nil
	}

	return "", nil, &net.DNSError{Name: name, Err: "no such host"}
}
