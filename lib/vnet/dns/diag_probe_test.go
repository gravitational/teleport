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

package dns

import "testing"

func TestHasDiagProbePrefix(t *testing.T) {
	cases := []struct {
		name string
		fqdn string
		want bool
	}{
		{name: "lowercase match", fqdn: "vnet-diag-abc.company.test.", want: true},
		{name: "uppercase match", fqdn: "VNET-DIAG-abc.company.test.", want: true},
		{name: "mixed case match", fqdn: "Vnet-Diag-abc.company.test.", want: true},
		{name: "exact prefix only", fqdn: "vnet-diag-", want: true},
		{name: "no random suffix", fqdn: "vnet-diag-.company.test.", want: true},
		{name: "regular app name", fqdn: "myapp.company.test.", want: false},
		{name: "similar prefix", fqdn: "vnet-other.company.test.", want: false},
		{name: "prefix in middle", fqdn: "x.vnet-diag-y.company.test.", want: false},
		{name: "empty string", fqdn: "", want: false},
		{name: "shorter than prefix", fqdn: "vnet-", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasDiagProbePrefix(tc.fqdn); got != tc.want {
				t.Errorf("HasDiagProbePrefix(%q) = %v, want %v", tc.fqdn, got, tc.want)
			}
		})
	}
}
