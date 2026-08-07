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

package beams

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

func TestVNetService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testYAMLCase[VNetServiceConfig]{
		{
			name: "full",
			in: VNetServiceConfig{
				Name:                "beams-vnet",
				DelegationSessionID: "8a50ba48-2fad-4c2c-a8ce-f48bc18db9ee",
				UpstreamNameservers: []string{
					"1.1.1.1:53",
					"[2606:4700:4700::1111]:53",
				},
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			name: "minimal",
			in: VNetServiceConfig{
				DelegationSessionID: "8a50ba48-2fad-4c2c-a8ce-f48bc18db9ee",
			},
		},
	}

	testYAML(t, tests)
}

func TestVNetService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testCheckAndSetDefaultsCase[*VNetServiceConfig]{
		{
			name: "valid",
			in: func() *VNetServiceConfig {
				return &VNetServiceConfig{
					DelegationSessionID: "8a50ba48-2fad-4c2c-a8ce-f48bc18db9ee",
				}
			},
		},
		{
			name: "valid upstream nameservers",
			in: func() *VNetServiceConfig {
				return &VNetServiceConfig{
					DelegationSessionID: "8a50ba48-2fad-4c2c-a8ce-f48bc18db9ee",
					UpstreamNameservers: []string{
						"1.1.1.1:53",
						"[2606:4700:4700::1111]:53",
					},
				}
			},
		},
		{
			name: "missing delegation_session_id",
			in: func() *VNetServiceConfig {
				return &VNetServiceConfig{}
			},
			wantErr: "delegation_session_id: is required",
		},
		{
			name: "invalid upstream nameserver",
			in: func() *VNetServiceConfig {
				return &VNetServiceConfig{
					DelegationSessionID: "8a50ba48-2fad-4c2c-a8ce-f48bc18db9ee",
					UpstreamNameservers: []string{"not-an-addr"},
				}
			},
			wantErr: "upstream_nameservers[0]: must be a valid `ip:port` pair",
		},
		{
			name:   "scoped",
			scoped: true,
			in: func() *VNetServiceConfig {
				return &VNetServiceConfig{
					DelegationSessionID: "8a50ba48-2fad-4c2c-a8ce-f48bc18db9ee",
				}
			},
			wantErr: "is not supported in scoped mode",
		},
	}

	testCheckAndSetDefaults(t, tests)
}
