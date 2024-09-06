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

func TestSPIFFESVIDOutput_YAML(t *testing.T) {
	t.Parallel()

	dest := &DestinationMemory{}
	tests := []testYAMLCase[SPIFFESVIDOutput]{
		{
			name: "full",
			in: SPIFFESVIDOutput{
				Destination: dest,
				SVID: SVIDRequest{
					Path: "/foo",
					Hint: "hint",
					SANS: SVIDRequestSANs{
						DNS: []string{"example.com"},
						IP:  []string{"10.0.0.1", "10.42.0.1"},
					},
				},
			},
		},
		{
			name: "minimal",
			in: SPIFFESVIDOutput{
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
	tests := []testCheckAndSetDefaultsCase[*SPIFFESVIDOutput]{
		{
			name: "valid",
			in: func() *SPIFFESVIDOutput {
				return &SPIFFESVIDOutput{
					Destination: memoryDestForTest(),
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
		},
		{
			name: "missing destination",
			in: func() *SPIFFESVIDOutput {
				return &SPIFFESVIDOutput{
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
			in: func() *SPIFFESVIDOutput {
				return &SPIFFESVIDOutput{
					Destination: memoryDestForTest(),
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
			in: func() *SPIFFESVIDOutput {
				return &SPIFFESVIDOutput{
					Destination: memoryDestForTest(),
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
			in: func() *SPIFFESVIDOutput {
				return &SPIFFESVIDOutput{
					Destination: memoryDestForTest(),
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
