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
)

func ptr[T any](v T) *T {
	return &v
}

func TestSPIFFEWorkloadAPIService_YAML(t *testing.T) {
	t.Parallel()

	tests := []testYAMLCase[SPIFFEWorkloadAPIService]{
		{
			name: "full",
			in: SPIFFEWorkloadAPIService{
				Listen: "unix:///var/run/spiffe.sock",
				SVIDs: []SVIDRequestWithRules{
					{
						SVIDRequest: SVIDRequest{
							Path: "/foo",
							Hint: "hint",
							SANS: SVIDRequestSANs{
								DNS: []string{"example.com"},
								IP:  []string{"10.0.0.1", "10.42.0.1"},
							},
						},
						Rules: []SVIDRequestRule{
							{
								Unix: SVIDRequestRuleUnix{
									PID: ptr(100),
									UID: ptr(1000),
									GID: ptr(1234),
								},
							},
							{
								Unix: SVIDRequestRuleUnix{
									PID: ptr(100),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "minimal",
			in: SPIFFEWorkloadAPIService{
				Listen: "unix:///var/run/spiffe.sock",
				SVIDs: []SVIDRequestWithRules{
					{
						SVIDRequest: SVIDRequest{
							Path: "/foo",
						},
					},
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestSPIFFEWorkloadAPIService_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	tests := []testCheckAndSetDefaultsCase[*SPIFFEWorkloadAPIService]{
		{
			name: "valid",
			in: func() *SPIFFEWorkloadAPIService {
				return &SPIFFEWorkloadAPIService{
					Listen: "unix:///var/run/spiffe.sock",
					SVIDs: []SVIDRequestWithRules{
						{
							SVIDRequest: SVIDRequest{
								Path: "/foo",
								Hint: "hint",
								SANS: SVIDRequestSANs{
									DNS: []string{"example.com"},
									IP:  []string{"10.0.0.1", "10.42.0.1"},
								},
							},
						},
					},
				}
			},
		},
		{
			name: "missing path",
			in: func() *SPIFFEWorkloadAPIService {
				return &SPIFFEWorkloadAPIService{
					Listen: "unix:///var/run/spiffe.sock",
					SVIDs: []SVIDRequestWithRules{
						{
							SVIDRequest: SVIDRequest{
								Path: "",
								Hint: "hint",
								SANS: SVIDRequestSANs{
									DNS: []string{"example.com"},
									IP:  []string{"10.0.0.1", "10.42.0.1"},
								},
							},
						},
					},
				}
			},
			wantErr: "svid.path: should not be empty",
		},
		{
			name: "path missing leading slash",
			in: func() *SPIFFEWorkloadAPIService {
				return &SPIFFEWorkloadAPIService{
					Listen: "unix:///var/run/spiffe.sock",
					SVIDs: []SVIDRequestWithRules{
						{
							SVIDRequest: SVIDRequest{
								Path: "foo",
								Hint: "hint",
								SANS: SVIDRequestSANs{
									DNS: []string{"example.com"},
									IP:  []string{"10.0.0.1", "10.42.0.1"},
								},
							},
						},
					},
				}
			},
			wantErr: "svid.path: should be prefixed with /",
		},
		{
			name: "missing listen addr",
			in: func() *SPIFFEWorkloadAPIService {
				return &SPIFFEWorkloadAPIService{
					Listen: "",
					SVIDs: []SVIDRequestWithRules{
						{
							SVIDRequest: SVIDRequest{
								Path: "foo",
								Hint: "hint",
								SANS: SVIDRequestSANs{
									DNS: []string{"example.com"},
									IP:  []string{"10.0.0.1", "10.42.0.1"},
								},
							},
						},
					},
				}
			},
			wantErr: "listen: should not be empty",
		},
		{
			name: "invalid ip",
			in: func() *SPIFFEWorkloadAPIService {
				return &SPIFFEWorkloadAPIService{
					Listen: "unix:///var/run/spiffe.sock",
					SVIDs: []SVIDRequestWithRules{
						{
							SVIDRequest: SVIDRequest{
								Path: "/foo",
								Hint: "hint",
								SANS: SVIDRequestSANs{
									DNS: []string{"example.com"},
									IP:  []string{"invalid ip"},
								},
							},
						},
					},
				}
			},
			wantErr: "ip_sans[0]: invalid IP address",
		},
	}
	testCheckAndSetDefaults(t, tests)
}
