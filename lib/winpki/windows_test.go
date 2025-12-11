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

package winpki

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func TestCRLDN(t *testing.T) {
	for _, test := range []struct {
		name        string
		clusterName string
		issuerSKID  []byte
		crlDN       string
		caType      types.CertAuthType
	}{
		{
			name:        "test cluster name",
			clusterName: "test",
			crlDN:       "CN=test,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			name:        "full cluster name",
			clusterName: "cluster.goteleport.com",
			crlDN:       "CN=cluster.goteleport.com,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			name:        "database CA",
			clusterName: "cluster.goteleport.com",
			caType:      types.DatabaseClientCA,
			crlDN:       "CN=cluster.goteleport.com,CN=TeleportDB,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			name:        "user CA",
			clusterName: "cluster.goteleport.com",
			caType:      types.UserCA,
			crlDN:       "CN=cluster.goteleport.com,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			name:        "user CA with SKID",
			clusterName: "example.com",
			caType:      types.UserCA,
			issuerSKID:  []byte{0x61, 0xbe, 0xe7, 0xf0, 0xb4, 0x88, 0x78, 0x33, 0x40, 0x7d, 0x7a, 0xc0, 0xa8, 0x2a, 0xeb, 0x3e, 0x9d, 0x9f, 0xa1, 0xba},
			crlDN:       "CN=C6VEFS5KH1S36G3TFB0AGANB7QEPV8DQ_example.com,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			name:        "user CA with SKID requiring padding",
			clusterName: "example.com",
			caType:      types.UserCA,
			// This test makes sure we don't corrupt the DN with base32 padding in the even the SKID is not exactly 20 bytes long.
			issuerSKID: []byte{0x61, 0xbe, 0xe7, 0xf0, 0xb4, 0x88, 0x78, 0x33, 0x40, 0x7d, 0x7a, 0xc0, 0xa8, 0x2a, 0xeb, 0x3e, 0x9d, 0x9f},
			crlDN:      "CN=C6VEFS5KH1S36G3TFB0AGANB7QEPU_example.com,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			name:        "long CN truncated",
			clusterName: "reallylongclustername.goteleport.com",
			caType:      types.UserCA,
			issuerSKID:  []byte{0x61, 0xbe, 0xe7, 0xf0, 0xb4, 0x88, 0x78, 0x33, 0x40, 0x7d, 0x7a, 0xc0, 0xa8, 0x2a, 0xeb, 0x3e, 0x9d, 0x9f, 0xa1, 0xba},
			crlDN:       "CN=C6VEFS5KH1S36G3TFB0AGANB7QEPV8DQ_reallylongclustern,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.crlDN, CRLDN(test.clusterName, test.issuerSKID, "test.goteleport.com", test.caType))
		})
	}
}

func TestConvertDistinguishedName(t *testing.T) {
	tests := []struct {
		dn   string
		want pkix.Name
	}{
		{
			"CN=a,DC=b,DC=c,DC=d", pkix.Name{ExtraNames: []pkix.AttributeTypeAndValue{
				{Type: asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 25}, Value: "d"},
				{Type: asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 25}, Value: "c"},
				{Type: asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 25}, Value: "b"},
				{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "a"},
			}},
		},
		{
			"CN=a,OU=b,DC=c", pkix.Name{ExtraNames: []pkix.AttributeTypeAndValue{
				{Type: asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 25}, Value: "c"},
				{Type: asn1.ObjectIdentifier{2, 5, 4, 11}, Value: "b"},
				{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "a"}},
			}},
		{
			"CN=a,CN=b,OU=c,C=d,DC=e,WRONG=wrong", pkix.Name{ExtraNames: []pkix.AttributeTypeAndValue{
				{Type: asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 25}, Value: "e"},
				{Type: asn1.ObjectIdentifier{2, 5, 4, 6}, Value: "d"},
				{Type: asn1.ObjectIdentifier{2, 5, 4, 11}, Value: "c"},
				{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "b"},
				{Type: asn1.ObjectIdentifier{2, 5, 4, 3}, Value: "a"}},
			}},
	}
	for _, tt := range tests {
		t.Run(tt.dn, func(t *testing.T) {
			got, err := convertDistinguishedName(tt.dn)
			require.NoError(t, err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertDistinguishedName() got = %v, want %v", got, tt.want)
			}
		})
	}
}
