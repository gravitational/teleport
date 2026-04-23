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

package subcav1

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"slices"

	"github.com/gravitational/trace"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/lib/tlsca"
)

func convertDistinguishedNameProto(dn *subcav1.DistinguishedName) (*pkix.Name, error) {
	switch {
	case dn == nil:
		return nil, nil
	case len(dn.Names) == 0:
		return nil, trace.BadParameter("empty distinguished name")
	}

	rdns := make(pkix.RDNSequence, 0, len(dn.Names))
	for i, atv := range dn.Names {
		switch {
		case len(atv.GetOid()) == 0:
			return nil, trace.BadParameter("names[%d]: empty OID", i)
		case atv.Value == nil:
			return nil, trace.BadParameter("names[%d]: empty Value", i)
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

	out := &pkix.Name{}
	out.FillFromRDNSequence(&rdns)
	return out, nil
}

func assignClusterNameToATVs(
	atvs []pkix.AttributeTypeAndValue,
	clusterName string,
) ([]pkix.AttributeTypeAndValue, error) {
	found := false
	for _, atv := range atvs {
		if !slices.Equal(atv.Type, tlsca.CAClusterNameExtensionOID) {
			continue
		}

		val, ok := atv.Value.(string)
		if !ok {
			return nil, trace.BadParameter(
				"OID %v is empty or not a string: %T",
				atv.Type,
				atv.Value,
			)
		}
		if val != clusterName {
			return nil, trace.BadParameter(
				"OID %v: incorrect cluster name: %s",
				atv.Type,
				atv.Value,
			)
		}

		found = true // Continue looping, we want to inspect all OIDs.
	}

	if !found {
		atvs = append(atvs, pkix.AttributeTypeAndValue{
			Type:  tlsca.CAClusterNameExtensionOID,
			Value: clusterName,
		})
	}

	return atvs, nil
}

func removeOID(
	atvs []pkix.AttributeTypeAndValue,
	oid asn1.ObjectIdentifier,
) []pkix.AttributeTypeAndValue {
	return slices.DeleteFunc(atvs, func(atv pkix.AttributeTypeAndValue) bool {
		return slices.Equal(atv.Type, oid)
	})
}
