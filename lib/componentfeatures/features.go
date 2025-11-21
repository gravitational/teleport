package componentfeatures

import (
	componentfeaturesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/componentfeatures/v1"
)

// FeatureID is used to wrap [componentfeaturesv1.ComponentFeatureID] for convenience methods.
type FeatureID int32

const (
	// FeatureUnspecified is the unspecified feature ID.
	FeatureUnspecified = FeatureID(
		componentfeaturesv1.ComponentFeatureID_COMPONENT_FEATURE_ID_UNSPECIFIED,
	)
	// FeatureResourceConstraintsV1 indicates support for Resource Constraints in Access Requests,
	// identity certificates, and AWS Console App resources.
	FeatureResourceConstraintsV1 = FeatureID(
		componentfeaturesv1.ComponentFeatureID_COMPONENT_FEATURE_ID_RESOURCE_CONSTRAINTS_V1,
	)
)

var featureIDToName = map[FeatureID]string{
	FeatureUnspecified:           "UNSPECIFIED",
	FeatureResourceConstraintsV1: "RESOURCE_CONSTRAINTS_V1",
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
