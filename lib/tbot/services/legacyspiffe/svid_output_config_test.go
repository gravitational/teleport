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

package legacyspiffe

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
)

func TestSPIFFESVIDOutput_YAML(t *testing.T) {
	t.Parallel()

	dest := &destination.Memory{}
	tests := []testYAMLCase[SVIDOutputConfig]{
		{
			name: "full",
			in: SVIDOutputConfig{
				Destination: dest,
				SVID: SVIDRequest{
					Path: "/foo",
					Hint: "hint",
					SANS: SVIDRequestSANs{
						DNS: []string{"example.com"},
						IP:  []string{"10.0.0.1", "10.42.0.1"},
					},
				},
				IncludeFederatedTrustBundles: true,
				JWTs: []JWTSVID{
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
			in: SVIDOutputConfig{
				Destination: dest,
				SVID: SVIDRequest{
					Path: "/foo",
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestSPIFFESVIDOutput_CheckAndSetDefaults(t *testing.T) {
	tests := []testCheckAndSetDefaultsCase[*SVIDOutputConfig]{
		{
			name: "valid",
			in: func() *SVIDOutputConfig {
				return &SVIDOutputConfig{
					Destination: destination.NewMemory(),
					SVID: SVIDRequest{
						Path: "/foo",
						Hint: "hint",
						SANS: SVIDRequestSANs{
							DNS: []string{"example.com"},
							IP:  []string{"10.0.0.1"},
						},
					},
					JWTs: []JWTSVID{
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
			in: func() *SVIDOutputConfig {
				return &SVIDOutputConfig{
					Destination: destination.NewMemory(),
					SVID: SVIDRequest{
						Path: "/foo",
						Hint: "hint",
						SANS: SVIDRequestSANs{
							DNS: []string{"example.com"},
							IP:  []string{"10.0.0.1"},
						},
					},
					JWTs: []JWTSVID{
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
			in: func() *SVIDOutputConfig {
				return &SVIDOutputConfig{
					Destination: destination.NewMemory(),
					SVID: SVIDRequest{
						Path: "/foo",
						Hint: "hint",
						SANS: SVIDRequestSANs{
							DNS: []string{"example.com"},
							IP:  []string{"10.0.0.1"},
						},
					},
					JWTs: []JWTSVID{
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
			in: func() *SVIDOutputConfig {
				return &SVIDOutputConfig{
					Destination: nil,
					SVID: SVIDRequest{
						Path: "/foo",
						Hint: "hint",
						SANS: SVIDRequestSANs{
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
			in: func() *SVIDOutputConfig {
				return &SVIDOutputConfig{
					Destination: destination.NewMemory(),
					SVID: SVIDRequest{
						Path: "",
						Hint: "hint",
						SANS: SVIDRequestSANs{
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
			in: func() *SVIDOutputConfig {
				return &SVIDOutputConfig{
					Destination: destination.NewMemory(),
					SVID: SVIDRequest{
						Path: "foo",
						Hint: "hint",
						SANS: SVIDRequestSANs{
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
			in: func() *SVIDOutputConfig {
				return &SVIDOutputConfig{
					Destination: destination.NewMemory(),
					SVID: SVIDRequest{
						Path: "/foo",
						Hint: "hint",
						SANS: SVIDRequestSANs{
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
