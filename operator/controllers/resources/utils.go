/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"fmt"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionReasonOriginLabelNotMatching = "OriginLabelNotMatching"
	ConditionReasonOriginLabelMatching    = "OriginLabelMatching"
	ConditionReasonNewResource            = "NewResource"
	ConditionReasonNoError                = "NoError"
	ConditionReasonTeleportError          = "TeleportError"
	ConditionTypeTeleportResourceOwned    = "TeleportResourceOwned"
	ConditionTypeSuccessfullyReconciled   = "SuccessfullyReconciled"
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
	if existingResource == nil {
		condition := metav1.Condition{
			Type:    ConditionTypeTeleportResourceOwned,
			Status:  metav1.ConditionTrue,
			Reason:  ConditionReasonNewResource,
			Message: "No existing Teleport resource found with that name. The created resource is owned by the operator.",
		}
		return condition, nil
	}
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

func getReconciliationCondition(err error) metav1.Condition {
	var condition metav1.Condition
	if err == nil {
		condition = metav1.Condition{
			Type:    ConditionTypeSuccessfullyReconciled,
			Status:  metav1.ConditionTrue,
			Reason:  ConditionReasonNoError,
			Message: "Teleport resource was successfully reconciled, no error was returned by Teleport.",
		}
	} else {
		condition = metav1.Condition{
			Type:    ConditionTypeSuccessfullyReconciled,
			Status:  metav1.ConditionFalse,
			Reason:  ConditionReasonTeleportError,
			Message: fmt.Sprintf("Teleport returned the error: %s", err),
		}
	}

	return condition
}
