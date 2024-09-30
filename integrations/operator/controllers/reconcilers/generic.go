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

	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gravitational/teleport/api/types"
)

// Resource is any Teleport Resource the controller manages.
type Resource any

// Adapter is an empty struct implementing helper functions for the reconciler
// to extract information from the Resource. This avoids having to implement the
// same interface on all resources. This became an issue as new resources are
// not implementing the types.Resource interface anymore.
type Adapter[T Resource] interface {
	GetResourceName(T) string
	GetResourceRevision(T) string
	GetResourceOrigin(T) string
	SetResourceRevision(T, string)
	SetResourceLabels(T, map[string]string)
}

// KubernetesCR is a Kubernetes CustomResource representing a Teleport Resource.
type KubernetesCR[T Resource] interface {
	kclient.Object
	ToTeleport() T
	StatusConditions() *[]v1.Condition
}

// resourceClient is a CRUD client for a specific Teleport Resource.
// Implementing this interface allows to be reconciled by the resourceReconciler
// instead of writing a new specific reconciliation loop.
// resourceClient implementations can optionally implement the resourceMutator
// and resourceMutator interfaces.
type resourceClient[T Resource] interface {
	Get(context.Context, string) (T, error)
	Create(context.Context, T) error
	Update(context.Context, T) error
	Delete(context.Context, string) error
}

// resourceMutator can be implemented by TeleportResourceClients
// to edit a Resource before its creation, or before its update based on the existing one.
type resourceMutator[T Resource] interface {
	Mutate(ctx context.Context, new, existing T, crKey kclient.ObjectKey) error
}

// resourceReconciler is a Teleport generic reconciler.
type resourceReconciler[T any, K KubernetesCR[T]] struct {
	ResourceBaseReconciler
	resourceClient resourceClient[T]
	gvk            schema.GroupVersionKind
	adapter        Adapter[T]
}

// Upsert is the resourceReconciler of the ResourceBaseReconciler UpsertExternal
// It contains the logic to check if the Resource already exists, if it is owned by the operator and what
// to do to reconcile the Teleport Resource based on the Kubernetes one.
func (r resourceReconciler[T, K]) Upsert(ctx context.Context, obj kclient.Object) error {
	debugLog := ctrllog.FromContext(ctx).V(1)
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("failed to convert Object into Resource object: %T", obj)
	}
	k8sResource := newKubeResource[K]()
	debugLog.Info("Converting resource from unstructured", "crType", reflect.TypeOf(k8sResource))

	// If an error happen we want to put it in status.conditions before returning.
	err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(
		u.Object,
		k8sResource,
		true, /* returnUnknownFields */
	)
	updateErr := updateStatus(updateStatusConfig{
		ctx:         ctx,
		client:      r.Client,
		k8sResource: k8sResource,
		condition:   getStructureConditionFromError(err),
	})
	if err != nil || updateErr != nil {
		return trace.NewAggregate(err, updateErr)
	}

	teleportResource := k8sResource.ToTeleport()

	debugLog.Info("Converting resource to teleport")
	name := r.adapter.GetResourceName(teleportResource)
	existingResource, err := r.resourceClient.Get(ctx, name)
	updateErr = updateStatus(updateStatusConfig{
		ctx:         ctx,
		client:      r.Client,
		k8sResource: k8sResource,
		condition:   getReconciliationConditionFromError(err, true /* ignoreNotFound */),
	})

	if err != nil && !trace.IsNotFound(err) || updateErr != nil {
		return trace.NewAggregate(err, updateErr)
	}
	// If err is nil, we found the Resource. If err != nil (and we did return), then the error was `NotFound`
	exists := err == nil

	if exists {
		debugLog.Info("Resource already exists")
		newOwnershipCondition, isOwned := r.checkOwnership(existingResource)
		debugLog.Info("Resource is owned")
		if updateErr = updateStatus(updateStatusConfig{
			ctx:         ctx,
			client:      r.Client,
			k8sResource: k8sResource,
			condition:   newOwnershipCondition,
		}); updateErr != nil {
			return trace.Wrap(updateErr)
		}
		if !isOwned {
			return trace.AlreadyExists("unowned Resource '%s' already exists", name)
		}
	} else {
		debugLog.Info("Resource does not exist yet")
		if updateErr = updateStatus(updateStatusConfig{
			ctx:         ctx,
			client:      r.Client,
			k8sResource: k8sResource,
			condition:   newResourceCondition,
		}); updateErr != nil {
			return trace.Wrap(updateErr)
		}
	}

	kubeLabels := obj.GetLabels()
	teleportLabels := make(map[string]string, len(kubeLabels)+1) // +1 because we'll add the origin label
	for k, v := range kubeLabels {
		teleportLabels[k] = v
	}
	teleportLabels[types.OriginLabel] = types.OriginKubernetes
	r.adapter.SetResourceLabels(teleportResource, teleportLabels)
	debugLog.Info("Propagating labels from kube resource", "kubeLabels", kubeLabels, "teleportLabels", teleportLabels)

	if mutator, ok := r.resourceClient.(resourceMutator[T]); ok {
		debugLog.Info("Mutating resource")
		objKey := kclient.ObjectKeyFromObject(k8sResource)
		if err := mutator.Mutate(ctx, teleportResource, existingResource, objKey); err != nil {
			// If an error happens we want to put it in status.conditions before returning.
			updateErr = updateStatus(updateStatusConfig{
				ctx:         ctx,
				client:      r.Client,
				k8sResource: k8sResource,
				condition: metav1.Condition{
					Type:    ConditionTypeSuccessfullyReconciled,
					Status:  metav1.ConditionFalse,
					Reason:  ConditionReasonMutationError,
					Message: fmt.Sprintf("The reconciliation failed, the operator failed to mutate the resource before creating it in Teleport. Mutation failed with error: %s", err),
				},
			})

			return trace.NewAggregate(err, updateErr)
		}
	}

	if !exists {
		// This is a new Resource
		err = r.resourceClient.Create(ctx, teleportResource)
	} else {
		// This is a Resource update, we must propagate the revision
		currentRevision := r.adapter.GetResourceRevision(existingResource)
		r.adapter.SetResourceRevision(teleportResource, currentRevision)
		debugLog.Info("Propagating revision", "currentRevision", currentRevision)

		err = r.resourceClient.Update(ctx, teleportResource)
	}
	// If an error happens we want to put it in status.conditions before returning.
	updateErr = updateStatus(updateStatusConfig{
		ctx:         ctx,
		client:      r.Client,
		k8sResource: k8sResource,
		condition:   getReconciliationConditionFromError(err, false /* ignoreNotFound */),
	})

	return trace.NewAggregate(err, updateErr)
}

// Delete is the resourceReconciler of the ResourceBaseReconciler DeleteExertal
func (r resourceReconciler[T, K]) Delete(ctx context.Context, obj kclient.Object) error {
	// This call catches non-existing resources or subkind mismatch (e.g. openssh nodes)
	// We can then check that we own the Resource before deleting it.
	resource, err := r.resourceClient.Get(ctx, obj.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	_, isOwned := r.checkOwnership(resource)
	if !isOwned {
		// The Resource doesn't belong to us, we bail out but unblock the CR deletion
		return nil
	}
	// This GET->check->DELETE dance is race-prone, but it's good enough for what
	// we want to do. No one should reconcile the same Resource as the operator.
	// If they do, it's their fault as the Resource was clearly flagged as belonging to us.
	return r.resourceClient.Delete(ctx, obj.GetName())
}

// Reconcile implements the controllers.Reconciler interface.
func (r resourceReconciler[T, K]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj, err := GetUnstructuredObjectFromGVK(r.gvk)
	if err != nil {
		return ctrl.Result{}, trace.Wrap(err, "creating object in which the CR will be unmarshalled")
	}
	return r.Do(ctx, req, obj)
}

// SetupWithManager implements the controllers.Reconciler interface.
func (r resourceReconciler[T, K]) SetupWithManager(mgr ctrl.Manager) error {
	// The resourceReconciler uses unstructured objects because of a silly json marshaling
	// issue. Teleport's utils.String is a list of strings, but marshals as a single string if there's a single item.
	// This is a questionable design as it breaks the openapi schema, but we're stuck with it. We had to relax openapi
	// validation in those CRD fields, and use an unstructured object for the client, else JSON unmarshalling fails.
	obj, err := GetUnstructuredObjectFromGVK(r.gvk)
	if err != nil {
		return trace.Wrap(err, "creating the model object for the manager watcher/client")
	}
	return ctrl.
		NewControllerManagedBy(mgr).
		For(obj).
		WithEventFilter(
			buildPredicate(),
		).
		Complete(r)
}

// isResourceOriginKubernetes reads a teleport Resource metadata, searches for the origin label and checks its
// value is kubernetes.
func (r resourceReconciler[T, K]) isResourceOriginKubernetes(resource T) bool {
	origin := r.adapter.GetResourceOrigin(resource)
	return origin == types.OriginKubernetes
}

// checkOwnership takes an existing Resource and validates the operator owns it.
// It returns an ownership condition and a boolean representing if the Resource is
// owned by the operator. The ownedResource must be non-nil.
func (r resourceReconciler[T, K]) checkOwnership(existingResource T) (metav1.Condition, bool) {
	if !r.isResourceOriginKubernetes(existingResource) {
		// Existing Teleport Resource does not belong to us, bailing out

		condition := metav1.Condition{
			Type:    ConditionTypeTeleportResourceOwned,
			Status:  metav1.ConditionFalse,
			Reason:  ConditionReasonOriginLabelNotMatching,
			Message: "A Resource with the same name already exists in Teleport and does not have the Kubernetes origin label. Refusing to reconcile.",
		}
		return condition, false
	}

	condition := metav1.Condition{
		Type:    ConditionTypeTeleportResourceOwned,
		Status:  metav1.ConditionTrue,
		Reason:  ConditionReasonOriginLabelMatching,
		Message: "Teleport Resource has the Kubernetes origin label.",
	}
	return condition, true
}

var newResourceCondition = metav1.Condition{
	Type:    ConditionTypeTeleportResourceOwned,
	Status:  metav1.ConditionTrue,
	Reason:  ConditionReasonNewResource,
	Message: "No existing Teleport Resource found with that name. The created Resource is owned by the operator.",
}
