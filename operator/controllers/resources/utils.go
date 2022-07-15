package resources

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionReasonOriginLabelNotMatching = "OriginLabelNotMatching"
	ConditionReasonOriginLabelMatching    = "OriginLabelMatching"
	ConditionReasonNewResource            = "NewResource"
	ConditionTypeTeleportResourceOwned    = "TeleportResourceOwned"
)

// isResourceOriginKubernetes reads a teleport resource metadata, searches for the origin label and checks its
// value is kubernetes.
func isResourceOriginKubernetes(resource types.Resource) bool {
	label := resource.GetMetadata().Labels[types.OriginLabel]
	return label == types.OriginKubernetes
}

// checkOwnership takes an existing resource and validates the operator owns it.
// It returns an ownership condition and an error if the resource is not owned by the operator
func checkOwnership(existingResource types.Resource) (metav1.Condition, error) {
	if existingResource != nil {
		if !isResourceOriginKubernetes(existingResource) {
			// Existing Teleport resource does not belong to us, bailing out

			condition := metav1.Condition{
				Type:    ConditionTypeTeleportResourceOwned,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonOriginLabelNotMatching,
				Message: "A resource with the same name already exists in Teleport and does not have the Kubernetes origin label. Refusing to reconcile.",
			}
			return condition, trace.AlreadyExists("unowned resource '%s' already exists", existingResource)
		}

		condition := metav1.Condition{
			Type:    ConditionTypeTeleportResourceOwned,
			Status:  metav1.ConditionTrue,
			Reason:  ConditionReasonOriginLabelMatching,
			Message: "Teleport resource has the Kubernetes origin label.",
		}
		return condition, nil
	}

	condition := metav1.Condition{
		Type:    ConditionTypeTeleportResourceOwned,
		Status:  metav1.ConditionTrue,
		Reason:  ConditionReasonNewResource,
		Message: "No existing Teleport resource found with that name. The created resource is owned by the operator.",
	}
	return condition, nil
}
