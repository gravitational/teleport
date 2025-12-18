// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package oraclejoin

import (
	"strings"

	"github.com/gravitational/trace"
	"github.com/oracle/oci-go-sdk/v65/common"
)

// Hack: StringToRegion will lazily load regions from a config file if its
// input isn't in its hard-coded list, in a non-threadsafe way. Call it here
// to load the config ahead of time so future calls are threadsafe.
var _ = common.StringToRegion("")

// ParseRegion parses a string into a full (not abbreviated) Oracle Cloud
// region. It returns the empty string if the input is not a valid region.
func ParseRegion(rawRegion string) (region, realm string) {
	canonicalRegion := common.StringToRegion(rawRegion)
	realm, err := canonicalRegion.RealmID()
	if err != nil {
		return "", ""
	}
	return string(canonicalRegion), realm
}

var ociRealms = map[string]struct{}{
	"oc1": {}, "oc2": {}, "oc3": {}, "oc4": {}, "oc8": {}, "oc9": {},
	"oc10": {}, "oc14": {}, "oc15": {}, "oc19": {}, "oc20": {}, "oc21": {},
	"oc23": {}, "oc24": {}, "oc26": {}, "oc29": {}, "oc35": {},
}

// ParseRegionFromOCID parses an Oracle OCID and returns the embedded region.
// It returns an error if the input is not a valid OCID.
func ParseRegionFromOCID(ocid string) (string, error) {
	// OCID format: ocid1.<RESOURCE TYPE>.<REALM>.[REGION][.FUTURE USE].<UNIQUE ID>
	// Check format.
	ocidParts := strings.Split(ocid, ".")
	switch len(ocidParts) {
	case 5, 6:
	default:
		return "", trace.BadParameter("not an ocid")
	}
	// Check version.
	if ocidParts[0] != "ocid1" {
		return "", trace.BadParameter("invalid ocid version: %v", ocidParts[0])
	}
	// Check realm.
	if _, ok := ociRealms[ocidParts[2]]; !ok {
		return "", trace.BadParameter("invalid realm: %v", ocidParts[2])
	}
	resourceType := ocidParts[1]
	region, realm := ParseRegion(ocidParts[3])
	// Check type. Only instance OCIDs should have a region.
	switch resourceType {
	case "instance":
		if region == "" {
			return "", trace.BadParameter("invalid region: %v", region)
		}
		if realm != ocidParts[2] {
			return "", trace.BadParameter("invalid realm %q for region %q", ocidParts[2], region)
		}
	case "compartment", "tenancy":
		if ocidParts[3] != "" {
			return "", trace.BadParameter("resource type %v should not have a region", resourceType)
		}
	default:
		return "", trace.BadParameter("unsupported resource type: %v", resourceType)
	}
	return region, nil
}
