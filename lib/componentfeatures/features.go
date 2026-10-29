/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package componentfeatures

import (
	componentfeaturesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/componentfeatures/v1"
	"github.com/gravitational/teleport/api/types"
)

// FeatureID is used to wrap [componentfeaturesv1.ComponentFeatureID] for convenience methods.
type FeatureID int32

const (
	// FeatureUnspecified is the unspecified feature ID.
	FeatureUnspecified FeatureID = iota
	// FeatureResourceConstraintsV1 indicates support for Resource Constraints in Access Requests,
	// identity certificates, and AWS Console App resources.
	FeatureResourceConstraintsV1
	// FeatureResourceConstraintsSSHV1 indicates support for enforcing SSH login
	// constraints on nodes.
	FeatureResourceConstraintsSSHV1
	// FeatureAppForwardedIdentityV1 indicates that an Application Service instance accepts a Proxy-forwarded client
	// identity for the application in its App Server record. See [SupportsAppForwardedIdentityV1].
	FeatureAppForwardedIdentityV1
)

var featureIDToName = map[FeatureID]string{
	FeatureUnspecified:              "unspecified feature",
	FeatureResourceConstraintsV1:    "AWS constraints",
	FeatureResourceConstraintsSSHV1: "SSH constraints",
	FeatureAppForwardedIdentityV1:   "forwarded app identities",
}

// String returns a short name for the FeatureID, falling back to the
// [componentfeaturesv1.ComponentFeatureID] enum name if not specified.
func (f FeatureID) String() string {
	if s, ok := featureIDToName[f]; ok {
		return s
	}
	if s, ok := componentfeaturesv1.ComponentFeatureID_name[int32(f)]; ok {
		return s
	}
	return "UNKNOWN"
}

// ToProto converts the FeatureID to its corresponding [componentfeaturesv1.ComponentFeatureID].
func (f FeatureID) ToProto() componentfeaturesv1.ComponentFeatureID {
	return componentfeaturesv1.ComponentFeatureID(f)
}

// RequiredFeatureForConstraints returns the component feature required to support rc.
// It returns false for unrecognized constraints.
func RequiredFeatureForConstraints(rc *types.ResourceConstraints) (id FeatureID, ok bool) {
	switch rc.GetDetails().(type) {
	case *types.ResourceConstraints_Ssh:
		return FeatureResourceConstraintsSSHV1, true
	case *types.ResourceConstraints_AwsConsole:
		return FeatureResourceConstraintsV1, true
	}
	return FeatureUnspecified, false
}
