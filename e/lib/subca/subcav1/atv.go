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

	"github.com/gravitational/teleport/lib/tlsca"
)

var organizationNameOIDType = asn1.ObjectIdentifier{2, 5, 4, 10}

func assignClusterNameToATVs(
	atvs []pkix.AttributeTypeAndValue,
	clusterName string,
) ([]pkix.AttributeTypeAndValue, error) {
	var hasOrgName, hasClusterName bool
	for _, atv := range atvs {
		switch {
		case atv.Type.Equal(organizationNameOIDType):
			if hasOrgName {
				// Only consider the first O=.
				// Teleport assumes the cluster name is the first occurrence.
				continue
			}
			hasOrgName = true
			if atv.Value == clusterName {
				hasClusterName = true
			}
		case atv.Type.Equal(tlsca.CAClusterNameExtensionOID):
			if atv.Value != clusterName {
				return nil, trace.BadParameter("OID %v: cluster name invalid or not a string: %v", atv.Type, atv.Value)
			}
			hasClusterName = true
		}

		// Continue looping, we want to inspect all OIDs.
	}

	if hasClusterName {
		return atvs, nil
	}

	var clusterNameType []int
	if !hasOrgName {
		// Favor O= for the cluster name instead of our custom OID.
		// Some downstream systems don't like custom OIDs (eg, Postgres).
		clusterNameType = organizationNameOIDType
	} else {
		clusterNameType = tlsca.CAClusterNameExtensionOID
	}
	return append(atvs, pkix.AttributeTypeAndValue{
		Type:  clusterNameType,
		Value: clusterName,
	}), nil
}

func removeOID(
	atvs []pkix.AttributeTypeAndValue,
	oid asn1.ObjectIdentifier,
) []pkix.AttributeTypeAndValue {
	return slices.DeleteFunc(atvs, func(atv pkix.AttributeTypeAndValue) bool {
		return slices.Equal(atv.Type, oid)
	})
}
