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

package subca

import (
	"crypto/x509/pkix"

	"github.com/gravitational/trace"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
)

// RDNSequenceToDistinguishedNameProto converts a pkix.Name to a
// subcav1.DistinguishedName proto.
//
// All ATV values must be strings. Always returns non-nil on success.
func RDNSequenceToDistinguishedNameProto(rdns pkix.RDNSequence) (*subcav1.DistinguishedName, error) {
	if len(rdns) == 0 {
		return &subcav1.DistinguishedName{}, nil
	}

	// Usually it's one item per nameset. OK if not.
	estimatedLen := len(rdns)

	dn := &subcav1.DistinguishedName{
		Names: make([]*subcav1.AttributeTypeAndValue, 0, estimatedLen),
	}
	for i, nameSet := range rdns {
		for j, atv := range nameSet {
			oid := make([]int32, len(atv.Type))
			for i, x := range atv.Type {
				// OID components are guaranteed to fit in a 32 bit integer as checked
				// by encoding/asn1.parseBase128Int.
				// https://cs.opensource.google/go/go/+/refs/tags/go1.26.2:src/encoding/asn1/asn1.go;l=323
				oid[i] = int32(x)
			}
			val, ok := atv.Value.(string)
			if !ok {
				return nil, trace.BadParameter(
					"rdns[%d][%d]: ATV value is not a string: %q=%v (%T)",
					i, j,
					atv.Type, atv.Value, atv.Value,
				)
			}
			dn.Names = append(dn.Names, &subcav1.AttributeTypeAndValue{
				Oid:   oid,
				Value: &val,
			})
		}
	}
	return dn, nil
}

// DistinguishedNameProtoToRDNSequence converts a subcav1.DistinguishedName
// proto to an RDNSequence.
//
// DistinguishedName protos are considered user-input, therefore this method
// applies more strict validation than its sibling
// [RDNSequenceToDistinguishedNameProto].
//
// A nil or empty DistinguishedName is considered invalid.
func DistinguishedNameProtoToRDNSequence(dn *subcav1.DistinguishedName) (pkix.RDNSequence, error) {
	if dn == nil || len(dn.Names) == 0 {
		return nil, trace.BadParameter("empty distinguished name")
	}

	rdns := make(pkix.RDNSequence, 0, len(dn.Names))
	for i, atv := range dn.Names {
		switch {
		case len(atv.GetOid()) == 0:
			return nil, trace.BadParameter("names[%d]: empty OID", i)
		case atv.Value == nil:
			return nil, trace.BadParameter("names[%d]: nil Value", i)
		}

		oid := make([]int, len(atv.Oid))
		for j, x := range atv.Oid {
			oid[j] = int(x)
		}
		rdns = append(rdns, pkix.RelativeDistinguishedNameSET{
			pkix.AttributeTypeAndValue{
				Type:  oid,
				Value: *atv.Value,
			},
		})
	}
	return rdns, nil
}
