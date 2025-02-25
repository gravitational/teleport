/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package reconcilers

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strconv"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	ConditionReasonFailedToDecode         = "FailedToDecode"
	ConditionReasonOriginLabelNotMatching = "OriginLabelNotMatching"
	ConditionReasonOriginLabelMatching    = "OriginLabelMatching"
	ConditionReasonNewResource            = "NewResource"
	ConditionReasonNoError                = "NoError"
	ConditionReasonTeleportError          = "TeleportError"
	ConditionReasonMutationError          = "MutationError"
	ConditionTypeTeleportResourceOwned    = "TeleportResourceOwned"
	ConditionTypeSuccessfullyReconciled   = "SuccessfullyReconciled"
	ConditionTypeValidStructure           = "ValidStructure"
)

// gvkFromScheme looks up the GVK from the runtime scheme.
// The structured type must have been registered before in the scheme. This function is used when you have a structured
// type, a scheme containing this structured type, and want to build an unstructured object for the same GVK.
func gvkFromScheme[K runtime.Object](scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
	structuredObj := newKubeResource[K]()
	gvks, _, err := scheme.ObjectKinds(structuredObj)
	if err != nil {
		return schema.GroupVersionKind{}, trace.Wrap(err, "looking up gvk in scheme for type %T", structuredObj)
	}
	if len(gvks) != 1 {
		return schema.GroupVersionKind{}, trace.CompareFailed(
			"failed GVK lookup in scheme, looked up %T and got %d matches, expected 1", structuredObj, len(gvks),
		)
	}
	return gvks[0], nil
}

// newKubeResource creates a new TeleportKubernetesResource
// the function supports structs or pointer to struct implementations of the TeleportKubernetesResource interface
func newKubeResource[K any]() K {
	// We create a new instance of K.
	var resource K
	// We take the type of K
	interfaceType := reflect.TypeOf(resource)
	// If K is not a pointer we don't need to do anything
	// If K is a pointer, new(K) is only initializing a nil pointer, we need to manually initialize its destination
	if interfaceType.Kind() == reflect.Ptr {
		// We create a new Value of the type pointed by K. reflect.New returns a pointer to this value
		initializedResource := reflect.New(interfaceType.Elem())
		// We cast back to K
		resource = initializedResource.Interface().(K)
	}
	return resource
}

// buildPredicate returns a predicate that triggers the reconciliation when:
// - the Resource generation changes
// - the Resource finalizers change
// - the Resource annotations change
// - the Resource labels change
// - the Resource is created
// - the Resource is deleted
// It does not trigger the reconciliation when:
// - the Resource status changes
func buildPredicate() predicate.Predicate {
	return predicate.Or(
		predicate.GenerationChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
		predicate.LabelChangedPredicate{},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				if e.ObjectOld == nil || e.ObjectNew == nil {
					return false
				}

				return !slices.Equal(e.ObjectNew.GetFinalizers(), e.ObjectOld.GetFinalizers())
			},
		},
	)
}

// getReconciliationConditionFromError takes an error returned by a call to Teleport and returns a
// metav1.Condition describing how the Teleport Resource reconciliation went. This is used to provide feedback to
// the user about the controller's ability to reconcile the Resource.
func getReconciliationConditionFromError(err error, ignoreNotFound bool) metav1.Condition {
	if err == nil || trace.IsNotFound(err) && ignoreNotFound {
		return metav1.Condition{
			Type:    ConditionTypeSuccessfullyReconciled,
			Status:  metav1.ConditionTrue,
			Reason:  ConditionReasonNoError,
			Message: "Teleport Resource was successfully reconciled, no error was returned by Teleport.",
		}
	}
	return metav1.Condition{
		Type:    ConditionTypeSuccessfullyReconciled,
		Status:  metav1.ConditionFalse,
		Reason:  ConditionReasonTeleportError,
		Message: fmt.Sprintf("Teleport returned the error: %s", err),
	}
}

// getStructureConditionFromError takes a conversion error from k8s apimachinery's runtime.UnstructuredConverter
// and returns a metav1.Condition describing how the status conversion went. This is used to provide feedback to
// the user about the controller's ability to reconcile the Resource.
func getStructureConditionFromError(err error) metav1.Condition {
	if err != nil {
		return metav1.Condition{
			Type:    ConditionTypeValidStructure,
			Status:  metav1.ConditionFalse,
			Reason:  ConditionReasonFailedToDecode,
			Message: fmt.Sprintf("Failed to decode Kubernetes CR: %s", err),
		}
	}
	return metav1.Condition{
		Type:    ConditionTypeValidStructure,
		Status:  metav1.ConditionTrue,
		Reason:  ConditionReasonNoError,
		Message: "Kubernetes CR was successfully decoded.",
	}
}

// updateStatusConfig is a configuration struct for silentUpdateStatus.
type updateStatusConfig struct {
	ctx         context.Context
	client      kclient.Client
	k8sResource interface {
		kclient.Object
		StatusConditions() *[]metav1.Condition
	}
	condition metav1.Condition
}

// updateStatus updates the Resource status but swallows the error if the update fails.
func updateStatus(config updateStatusConfig) error {
	// If the condition is empty, we don't want to update the status.
	if config.condition == (metav1.Condition{}) {
		return nil
	}
	log := ctrllog.FromContext(config.ctx)
	meta.SetStatusCondition(
		config.k8sResource.StatusConditions(),
		config.condition,
	)
	statusErr := config.client.Status().Update(config.ctx, config.k8sResource)
	if statusErr != nil {
		log.Error(statusErr, "failed to report error in status conditions")
	}
	return trace.Wrap(statusErr)
}

// GetUnstructuredObjectFromGVK creates a new empty unstructured object with the
// given Group Version and Kind.
func GetUnstructuredObjectFromGVK(gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
	if gvk.Empty() {
		return nil, trace.BadParameter("cannot create an object for an empty GVK, aborting")
	}
	obj := unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	return &obj, nil
}

// checkAnnotationFlag checks is the Kubernetes Resource is annotated with a
// flag and parses its value. Returns the default value if the flag is missing
// or the annotation value cannot be parsed.
func checkAnnotationFlag(object kclient.Object, flagName string, defaultValue bool) bool {
	annotation, ok := object.GetAnnotations()[flagName]
	if !ok {
		return defaultValue
	}
	value, err := strconv.ParseBool(annotation)
	if err != nil {
		return defaultValue
	}
	return value
}
