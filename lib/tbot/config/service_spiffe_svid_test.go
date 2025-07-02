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

package config

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/services/legacyspiffe"
)

func TestSPIFFESVIDOutput_YAML(t *testing.T) {
	t.Parallel()

	dest := &destination.Memory{}
	tests := []testYAMLCase[legacyspiffe.SVIDOutputConfig]{
		{
			name: "full",
			in: legacyspiffe.SVIDOutputConfig{
				Destination: dest,
				SVID: legacyspiffe.SVIDRequest{
					Path: "/foo",
					Hint: "hint",
					SANS: legacyspiffe.SVIDRequestSANs{
						DNS: []string{"example.com"},
						IP:  []string{"10.0.0.1", "10.42.0.1"},
					},
				},
				IncludeFederatedTrustBundles: true,
				JWTs: []legacyspiffe.JWTSVID{
					{
						Audience: "example.com",
						FileName: "foo",
					},
					{
						Audience: "2.example.com",
						FileName: "bar",
					},
				},
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
			},
		},
		{
			name: "minimal",
			in: legacyspiffe.SVIDOutputConfig{
				Destination: dest,
				SVID: legacyspiffe.SVIDRequest{
					Path: "/foo",
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestSPIFFESVIDOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*legacyspiffe.SVIDOutputConfig]{
		{
			name: "valid",
			in: func() *legacyspiffe.SVIDOutputConfig {
				return &legacyspiffe.SVIDOutputConfig{
					Destination: memoryDestForTest(),
					SVID: legacyspiffe.SVIDRequest{
						Path: "/foo",
						Hint: "hint",
						SANS: legacyspiffe.SVIDRequestSANs{
							DNS: []string{"example.com"},
							IP:  []string{"10.0.0.1"},
						},
					},
					JWTs: []legacyspiffe.JWTSVID{
						{
							FileName: "foo",
							Audience: "example.com",
						},
					},
				}
			},
		},
		{
			name: "missing jwt name",
			in: func() *legacyspiffe.SVIDOutputConfig {
				return &legacyspiffe.SVIDOutputConfig{
					Destination: memoryDestForTest(),
					SVID: legacyspiffe.SVIDRequest{
						Path: "/foo",
						Hint: "hint",
						SANS: legacyspiffe.SVIDRequestSANs{
							DNS: []string{"example.com"},
							IP:  []string{"10.0.0.1"},
						},
					},
					JWTs: []legacyspiffe.JWTSVID{
						{
							Audience: "example.com",
						},
					},
				}
			},
			wantErr: "name: should not be empty",
		},
		{
			name: "missing jwt audience",
			in: func() *legacyspiffe.SVIDOutputConfig {
				return &legacyspiffe.SVIDOutputConfig{
					Destination: memoryDestForTest(),
					SVID: legacyspiffe.SVIDRequest{
						Path: "/foo",
						Hint: "hint",
						SANS: legacyspiffe.SVIDRequestSANs{
							DNS: []string{"example.com"},
							IP:  []string{"10.0.0.1"},
						},
					},
					JWTs: []legacyspiffe.JWTSVID{
						{
							FileName: "foo",
						},
					},
				}
			},
			wantErr: "audience: should not be empty",
		},
		{
			name: "missing destination",
			in: func() *legacyspiffe.SVIDOutputConfig {
				return &legacyspiffe.SVIDOutputConfig{
					Destination: nil,
					SVID: legacyspiffe.SVIDRequest{
						Path: "/foo",
						Hint: "hint",
						SANS: legacyspiffe.SVIDRequestSANs{
							DNS: []string{"example.com"},
							IP:  []string{"10.0.0.1"},
						},
					},
				}
			},
			wantErr: "no destination configured for output",
		},
		{
			name: "missing path",
			in: func() *legacyspiffe.SVIDOutputConfig {
				return &legacyspiffe.SVIDOutputConfig{
					Destination: memoryDestForTest(),
					SVID: legacyspiffe.SVIDRequest{
						Path: "",
						Hint: "hint",
						SANS: legacyspiffe.SVIDRequestSANs{
							DNS: []string{"example.com"},
							IP:  []string{"10.0.0.1"},
						},
					},
				}
			},
			wantErr: "svid.path: should not be empty",
		},
		{
			name: "path missing leading slash",
			in: func() *legacyspiffe.SVIDOutputConfig {
				return &legacyspiffe.SVIDOutputConfig{
					Destination: memoryDestForTest(),
					SVID: legacyspiffe.SVIDRequest{
						Path: "foo",
						Hint: "hint",
						SANS: legacyspiffe.SVIDRequestSANs{
							DNS: []string{"example.com"},
							IP:  []string{"10.0.0.1"},
						},
					},
				}
			},
			wantErr: "svid.path: should be prefixed with /",
		},
		{
			name: "invalid ip",
			in: func() *legacyspiffe.SVIDOutputConfig {
				return &legacyspiffe.SVIDOutputConfig{
					Destination: memoryDestForTest(),
					SVID: legacyspiffe.SVIDRequest{
						Path: "/foo",
						Hint: "hint",
						SANS: legacyspiffe.SVIDRequestSANs{
							DNS: []string{"example.com"},
							IP:  []string{"thisisntanip"},
						},
					},
				}
			},
			wantErr: "ip_sans[0]: invalid IP address",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
