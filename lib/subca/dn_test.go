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

package subca_test

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/lib/subca"
)

func TestDistinguishedName_roundtrip(t *testing.T) {
	t.Parallel()

	want := (pkix.Name{
		Organization: []string{"Llama"},
		CommonName:   "Llamo",
		ExtraNames: []pkix.AttributeTypeAndValue{
			{Type: asn1.ObjectIdentifier{1, 2, 3, 4}, Value: "alpaca"},
		},
	}).ToRDNSequence()

	dn, err := subca.RDNSequenceToDistinguishedNameProto(want)
	require.NoError(t, err, "RDNSequenceToDistinguishedNameProto errored")

	got, err := subca.DistinguishedNameProtoToRDNSequence(dn)
	require.NoError(t, err, "DistinguishedNameProtoToRDNSequence errored")
	assert.Equal(t, want, got, "DistinguishedName roundtrip mismatch")
}

func TestRDNSequenceToDistinguishedNameProto(t *testing.T) {
	t.Parallel()

	// Success tested by TestDistinguishedName_roundtrip.

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		got, err := subca.RDNSequenceToDistinguishedNameProto(nil)
		require.NoError(t, err)
		want := &subcav1.DistinguishedName{}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("DistinguishedName mismatch (-want +got)\n%s", diff)
		}
	})

	t.Run("OID component is MaxInt32", func(t *testing.T) {
		t.Parallel()

		val := "llama"
		got, err := subca.RDNSequenceToDistinguishedNameProto(pkix.RDNSequence{
			{{Type: asn1.ObjectIdentifier{99, 1, 2, math.MaxInt32}, Value: val}},
		})
		require.NoError(t, err)

		want := &subcav1.DistinguishedName{
			Names: []*subcav1.AttributeTypeAndValue{
				{Oid: []int32{99, 1, 2, int32(math.MaxInt32)}, Value: &val},
			},
		}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("DistinguishedName mismatch (-want +got)\n%s", diff)
		}
	})

	tests := []struct {
		name    string
		rdns    pkix.RDNSequence
		wantErr string
	}{
		{
			name: "OID negative",
			rdns: pkix.RDNSequence{
				{{Type: asn1.ObjectIdentifier{99, 1, 2, -1}, Value: "llama"}},
			},
			wantErr: "OID component out of bounds",
		},
		{
			name: "OID out of bounds",
			rdns: pkix.RDNSequence{
				{{Type: asn1.ObjectIdentifier{99, 1, 2, math.MaxInt32 + 1}, Value: "llama"}},
			},
			wantErr: "OID component out of bounds",
		},
		{
			name: "non-string value",
			rdns: pkix.RDNSequence{
				{
					{Type: asn1.ObjectIdentifier{1, 2, 3, 4}, Value: "ok"}, // OK
					{Type: asn1.ObjectIdentifier{1, 2, 3, 5}, Value: 42},   // NOK
				},
			},
			wantErr: "not a string",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := subca.RDNSequenceToDistinguishedNameProto(test.rdns)
			assert.ErrorContains(t, err, test.wantErr, "error mismatch")
		})
	}
}

func TestDistinguishedNameProtoToRDNSequence(t *testing.T) {
	t.Parallel()

	// Success tested by TestDistinguishedName_roundtrip.

	exampleValue := "alpaca"

	tests := []struct {
		name    string
		dn      *subcav1.DistinguishedName
		wantErr string
	}{
		{
			name:    "nil",
			wantErr: "empty distinguished name",
		},
		{
			name:    "empty",
			dn:      &subcav1.DistinguishedName{},
			wantErr: "empty distinguished name",
		},
		{
			name: "ATV nil",
			dn: &subcav1.DistinguishedName{
				Names: []*subcav1.AttributeTypeAndValue{
					nil,
				},
			},
			wantErr: "empty OID",
		},
		{
			name: "ATV.OID empty",
			dn: &subcav1.DistinguishedName{
				Names: []*subcav1.AttributeTypeAndValue{
					{Oid: []int32{}, Value: &exampleValue},
				},
			},
			wantErr: "empty OID",
		},
		{
			name: "ATV.OID negative",
			dn: &subcav1.DistinguishedName{
				Names: []*subcav1.AttributeTypeAndValue{
					{Oid: []int32{1, 2, 3, -1}, Value: &exampleValue},
				},
			},
			wantErr: "OID component out of bounds",
		},
		{
			name: "ATV.Value nil",
			dn: &subcav1.DistinguishedName{
				Names: []*subcav1.AttributeTypeAndValue{
					{Oid: []int32{1, 2, 3, 4}},
				},
			},
			wantErr: "nil Value",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := subca.DistinguishedNameProtoToRDNSequence(test.dn)
			assert.ErrorContains(t, err, test.wantErr, "error mismatch")
		})
	}
}
