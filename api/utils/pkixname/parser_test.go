// Copyright 2025 Gravitational, Inc.
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

package pkixname_test

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/pkixname"
)

func TestParseDistinguishedName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       string
		want     *pkix.Name
		wantErr  string // error substring
		skipGoDN bool
	}{
		{
			name: "common attributes",
			in:   "SERIALNUMBER=12345,CN=Teleport CA,OU=Core,O=Teleport,POSTALCODE=94612,STREET=2100 Franklin St,L=Oakland,ST=California,C=US",
			want: &pkix.Name{
				Country:            []string{"US"},
				Organization:       []string{"Teleport"},
				OrganizationalUnit: []string{"Core"},
				Locality:           []string{"Oakland"},
				Province:           []string{"California"},
				StreetAddress:      []string{"2100 Franklin St"},
				PostalCode:         []string{"94612"},
				SerialNumber:       "12345",
				CommonName:         "Teleport CA",
			},
		},
		{
			name: "escaped characters",
			in:   `CN=\,\=\+\<\>\#\;\\\"`,
			want: &pkix.Name{
				CommonName: `,=+<>#;\"`,
			},
		},
		{
			name: "escaped spaces",
			in:   `CN=\   Llama CA  \ `,
			want: &pkix.Name{
				CommonName: "   Llama CA   ",
			},
		},
		{
			name:    "invalid <",
			in:      "CN=<",
			wantErr: "not quoted",
		},
		{
			name:    "invalid >",
			in:      "CN=>",
			wantErr: "not quoted",
		},
		{
			name: "quoted string",
			in:   `CN=   "  Llama  CA  ,=+<>#;\\'\"  "  `,
			want: &pkix.Name{
				CommonName: `  Llama  CA  ,=+<>#;\'"  `,
			},
		},
		{
			name: "quoted string transitions",
			in:   `C="US"+C="CA",CN="Bar"`,
			want: &pkix.Name{
				Country:    []string{"US", "CA"},
				CommonName: "Bar",
			},
		},
		{
			name: "string with trailing spaces transitions",
			in:   `C=US + C=CA , CN=Bar  `,
			want: &pkix.Name{
				Country:    []string{"US", "CA"},
				CommonName: "Bar",
			},
		},
		{
			name: "semicolon",
			in:   "O=Llama;OU=Core Team;CN=Llama Core Team CA",
			want: &pkix.Name{
				Organization:       []string{"Llama"},
				OrganizationalUnit: []string{"Core Team"},
				CommonName:         "Llama Core Team CA",
			},
		},
		{
			name: "multi-valued RDN",
			in:   "O=Teleport+O=Llama+O=Llama with spaces,CN=Teleport Llama CA",
			want: &pkix.Name{
				Organization: []string{"Teleport", "Llama", "Llama with spaces"},
				CommonName:   "Teleport Llama CA",
			},
		},
		{
			name: "custom OIDs",
			in:   "1=Foo,1.2=Bar,1.2.3.4.5.6.7=Baz",
			want: &pkix.Name{
				ExtraNames: []pkix.AttributeTypeAndValue{
					{
						Type:  asn1.ObjectIdentifier{1},
						Value: "Foo",
					},
					{
						Type:  asn1.ObjectIdentifier{1, 2},
						Value: "Bar",
					},
					{
						Type:  asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 7},
						Value: "Baz",
					},
				},
			},
			skipGoDN: true, // outputs custom OIDs as hexstring
		},
		{
			name: "empty string",
			in:   "",
			want: &pkix.Name{},
		},
		{
			name: "whitespaces string",
			in:   "   ",
			want: &pkix.Name{},
		},
		{
			name: "empty value",
			in:   "CN=, C = ,O=Llama,OU=",
			want: &pkix.Name{
				Country:            []string{""},
				Organization:       []string{"Llama"},
				OrganizationalUnit: []string{""},
			},
		},
		{
			name: "empty quoted value",
			in:   `CN="" , C = "" ,O="Llama",OU=""`,
			want: &pkix.Name{
				Country:            []string{""},
				Organization:       []string{"Llama"},
				OrganizationalUnit: []string{""},
			},
		},
		{
			name: "whitespaces",
			in:   `  CN  =  Llama CA  ,  O  =  "Llama"  +  O  =  Teleport  `,
			want: &pkix.Name{
				Organization: []string{"Llama", "Teleport"},
				CommonName:   "Llama CA",
			},
		},

		{
			name:    "malformed DN: incomplete ATV",
			in:      `C`,
			wantErr: "want attributeType",
		},
		{
			name:    "malformed DN: incomplete ATV 2",
			in:      `C  `,
			wantErr: "want '=' attributeValue",
		},
		{
			name:    "malformed DN: incomplete ATV 3",
			in:      `C,`,
			wantErr: "want attributeType or '='",
		},
		{
			name:    "malformed DN: incomplete ATV 4",
			in:      `C  ,`,
			wantErr: "want '=' attributeValue",
		},
		{
			name:    "malformed DN: invalid start",
			in:      `+C=1`,
			wantErr: "want attributeType",
		},
		{
			name:    "malformed DN: invalid start 2",
			in:      `,C=1`,
			wantErr: "want attributeType",
		},
		{
			name:    "malformed DN: invalid start 3",
			in:      `=C=1`,
			wantErr: "want attributeType",
		},
		{
			name:    "malformed DN: invalid attributeValue",
			in:      `C=<`,
			wantErr: "special character",
		},
		{
			name:    "malformed DN: missing =",
			in:      `C+1`,
			wantErr: "want attributeType or '='",
		},
		{
			name:    "malformed second DN: trailing ,",
			in:      `C=1,`,
			wantErr: "want attributeType, found EOF",
		},
		{
			name:    "malformed second DN: empty string and trailing +",
			in:      `C=+`,
			wantErr: "want attributeType",
		},
		{
			name:    "malformed second DN: empty string and trailing ,",
			in:      `C=,`,
			wantErr: "want attributeType",
		},
		{
			name:    "malformed second DN: empty string and trailing ;",
			in:      `C=;`,
			wantErr: "want attributeType",
		},
		{
			name:    "malformed DN: string extravaganza",
			in:      `C=Foo " Bar " Baz`,
			wantErr: "special character",
		},
		{
			name:    "malformed DN: string extravaganza 2",
			in:      `C="Foo " Bar "Baz"`,
			wantErr: "want '+' or ','",
		},
		{
			name:    "malformed DN: incomplete escape",
			in:      `CN=\`,
			wantErr: "want escaped character",
		},
		{
			name:    "malformed DN: incomplete quote",
			in:      `CN="`,
			wantErr: "want closing quote",
		},
		{
			name:    "malformed DN: quoted attribute",
			in:      `"CN"=Foo`,
			wantErr: "want attributeType",
		},
		{
			name:    "malformed DN: invalid attributeType charset",
			in:      "L.LAMA=Bar", // letter start means no dots allowed
			wantErr: "invalid attributeType",
		},
		{
			name:    "malformed DN: invalid attributeType charset 2",
			in:      "1LLAMA=Bar", // number start not allowed
			wantErr: "invalid attributeType",
		},
		{
			name:    "malformed DN: invalid oid",
			in:      "1.9999999999999999999.3=Bar", // too large for int
			wantErr: "cannot parse OID",
		},
		{
			name:    "invalid: repeated attributeType",
			in:      "C=US,C=CA",
			wantErr: "repeated attributeType",
		},
		{
			name:    "invalid: unknown attributeType",
			in:      "LLAMA=Yes",
			wantErr: "unknown attributeType",
		},

		{
			name: "rfc2253 example1",
			in:   `CN=Steve Kille,O=Isode Limited,C=GB`,
			want: &pkix.Name{
				Country:      []string{"GB"},
				Organization: []string{"Isode Limited"},
				CommonName:   "Steve Kille",
			},
		},
		{
			name: "rfc2253 example2",
			// We don't allow OU+CN, it has to be the same attr.
			in:      `OU=Sales+CN=J. Smith,O=Widget Inc.,C=US`,
			wantErr: "multi-valued RDN",
		},
		{
			name: "rfc2253 example3",
			in:   `CN=L. Eagle,O=Sue\, Grabbit and Runn,C=GB`,
			want: &pkix.Name{
				Country:      []string{"GB"},
				Organization: []string{"Sue, Grabbit and Runn"},
				CommonName:   "L. Eagle",
			},
		},
		{
			name:    "rfc2253 example4",
			in:      `CN=Before\0DAfter,O=Test,C=GB`, // We don't allow hex escapes.
			wantErr: "unexpected escaped character",
		},
		{
			name:    "rfc2253 example5",
			in:      `1.3.6.1.4.1.1466.0=#04024869,O=Test,C=GB`,
			wantErr: "hexstring not supported",
		},
		{
			name:    "rfc2253 example6",
			in:      `SN=Lu\C4\8Di\C4\87`, // We don't allow hex escapes.
			wantErr: "unexpected escaped character",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := pkixname.ParseDistinguishedName(test.in)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "ParseDistinguishedName() error mismatch")
				return
			}
			require.NoError(t, err, "ParseDistinguishedName() errored")
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("pkix.Name mismatch (-want +got)\n%s", diff)
			}

			if test.skipGoDN {
				return
			}

			// If Go generates a different string, parse it too.
			// The end result is equivalent.
			goDN := test.want.String()
			if goDN != test.in {
				t.Run("Go's DN", func(t *testing.T) {
					t.Logf("DN: [%s]", goDN)

					got, err := pkixname.ParseDistinguishedName(goDN)
					require.NoError(t, err, "ParseDistinguishedName() errored")
					if diff := cmp.Diff(test.want, got); diff != "" {
						t.Errorf("pkix.Name mismatch (-want +got)\n%s", diff)
					}
				})
			}
		})
	}
}
