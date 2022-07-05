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
	metadata := resource.GetMetadata()
	if label, ok := metadata.Labels[types.OriginLabel]; ok {
		return label == types.OriginKubernetes
	}
	return false
}

func checkOwnership(existingResource types.Resource, setCondition func(condition metav1.Condition) error) error {
	if existingResource != nil {
		if !isResourceOriginKubernetes(existingResource) {
			// Existing Teleport resource does not belong to us, bailing out

			condition := metav1.Condition{
				Type:    ConditionTypeTeleportResourceOwned,
				Status:  metav1.ConditionFalse,
				Reason:  ConditionReasonOriginLabelNotMatching,
				Message: "A resource with the same name already exists in Teleport and does not have the Kubernetes origin label. Refusing to reconcile.",
			}
			err := setCondition(condition)
			if err != nil {
				return trace.Wrap(err)
			}
			return trace.AlreadyExists("unowned resource '%s' already exists", existingResource)
		}

		condition := metav1.Condition{
			Type:    ConditionTypeTeleportResourceOwned,
			Status:  metav1.ConditionTrue,
			Reason:  ConditionReasonOriginLabelMatching,
			Message: "Teleport resource has the Kubernetes origin label.",
		}
		err := setCondition(condition)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	condition := metav1.Condition{
		Type:    ConditionTypeTeleportResourceOwned,
		Status:  metav1.ConditionTrue,
		Reason:  ConditionReasonNewResource,
		Message: "No existing Teleport resource found with that name. The created resource is owned by the operator.",
	}
	err := setCondition(condition)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
