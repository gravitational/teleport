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
	"context"
	"reflect"
	"slices"

	"github.com/gravitational/trace"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gravitational/teleport/api/types"
)

type TeleportResource interface {
	GetName() string
	SetOrigin(string)
	GetMetadata() types.Metadata
	GetRevision() string
	SetRevision(string)
}

// TeleportKubernetesResource is a Kubernetes resource representing a Teleport resource
type TeleportKubernetesResource[T TeleportResource] interface {
	kclient.Object
	ToTeleport() T
	StatusConditions() *[]v1.Condition
}

// TeleportResourceReconciler is a Teleport generic reconciler. It reconciles TeleportKubernetesResource
// with Teleport's types.ResourceWithOrigin
type TeleportResourceReconciler[T TeleportResource, K TeleportKubernetesResource[T]] struct {
	ResourceBaseReconciler
	resourceClient TeleportResourceClient[T]
}

// TeleportResourceClient is a CRUD client for a specific Teleport resource.
// Implementing this interface allows to be reconciled by the TeleportResourceReconciler
// instead of writing a new specific reconciliation loop.
// TeleportResourceClient implementations can optionally implement TeleportResourceMutator
type TeleportResourceClient[T TeleportResource] interface {
	Get(context.Context, string) (T, error)
	Create(context.Context, T) error
	Update(context.Context, T) error
	Delete(context.Context, string) error
}

// TeleportResourceMutator can be implemented by TeleportResourceClients
// to edit a resource before its creation/update.
type TeleportResourceMutator[T TeleportResource] interface {
	Mutate(new T)
}

// TeleportExistingResourceMutator can be implemented by TeleportResourceClients
// to edit a resource before its update based on the existing one.
type TeleportExistingResourceMutator[T TeleportResource] interface {
	MutateExisting(new, existing T)
}

// NewTeleportResourceReconciler instanciates a TeleportResourceReconciler from a TeleportResourceClient.
func NewTeleportResourceReconciler[T TeleportResource, K TeleportKubernetesResource[T]](
	client kclient.Client,
	resourceClient TeleportResourceClient[T],
) *TeleportResourceReconciler[T, K] {
	reconciler := &TeleportResourceReconciler[T, K]{
		ResourceBaseReconciler: ResourceBaseReconciler{Client: client},
		resourceClient:         resourceClient,
	}
	reconciler.ResourceBaseReconciler.UpsertExternal = reconciler.Upsert
	reconciler.ResourceBaseReconciler.DeleteExternal = reconciler.Delete
	return reconciler
}

// Upsert is the TeleportResourceReconciler of the ResourceBaseReconciler UpsertExertal
// It contains the logic to check if the resource already exists, if it is owned by the operator and what
// to do to reconcile the Teleport resource based on the Kubernetes one.
func (r TeleportResourceReconciler[T, K]) Upsert(ctx context.Context, obj kclient.Object) error {
	k8sResource, ok := obj.(K)
	if !ok {
		return trace.BadParameter("failed to convert Object into resource object: %T", obj)
	}
	teleportResource := k8sResource.ToTeleport()

	existingResource, err := r.resourceClient.Get(ctx, teleportResource.GetName())
	updateErr := updateStatus(updateStatusConfig{
		ctx:         ctx,
		client:      r.Client,
		k8sResource: k8sResource,
		condition:   getReconciliationConditionFromError(err, true /* ignoreNotFound */),
	})

	if err != nil && !trace.IsNotFound(err) || updateErr != nil {
		return trace.NewAggregate(err, updateErr)
	}
	// If err is nil, we found the resource. If err != nil (and we did return), then the error was `NotFound`
	exists := err == nil

	if exists {
		newOwnershipCondition, isOwned := checkOwnership(existingResource)
		if updateErr = updateStatus(updateStatusConfig{
			ctx:         ctx,
			client:      r.Client,
			k8sResource: k8sResource,
			condition:   newOwnershipCondition,
		}); updateErr != nil {
			return trace.Wrap(updateErr)
		}
		if !isOwned {
			return trace.AlreadyExists("unowned resource '%s' already exists", existingResource.GetName())
		}
	} else {
		if updateErr = updateStatus(updateStatusConfig{
			ctx:         ctx,
			client:      r.Client,
			k8sResource: k8sResource,
			condition:   newResourceCondition,
		}); updateErr != nil {
			return trace.Wrap(updateErr)
		}
	}

	teleportResource.SetOrigin(types.OriginKubernetes)

	if !exists {
		// This is a new resource
		if mutator, ok := r.resourceClient.(TeleportResourceMutator[T]); ok {
			mutator.Mutate(teleportResource)
		}

		err = r.resourceClient.Create(ctx, teleportResource)
	} else {
		// This is a resource update, we must propagate the revision
		teleportResource.SetRevision(existingResource.GetRevision())
		if mutator, ok := r.resourceClient.(TeleportExistingResourceMutator[T]); ok {
			mutator.MutateExisting(teleportResource, existingResource)
		}

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

// Delete is the TeleportResourceReconciler of the ResourceBaseReconciler DeleteExertal
func (r TeleportResourceReconciler[T, K]) Delete(ctx context.Context, obj kclient.Object) error {
	// This call catches non-existing resources or subkind mismatch (e.g. openssh nodes)
	// We can then check that we own the resource before deleting it.
	resource, err := r.resourceClient.Get(ctx, obj.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	_, isOwned := checkOwnership(resource)
	if !isOwned {
		// The resource doesn't belong to us, we bail out but unblock the CR deletion
		return nil
	}
	// This GET->check->DELETE dance is race-prone, but it's good enough for what
	// we want to do. No one should reconcile the same resource as the operator.
	// If they do, it's their fault as the resource was clearly flagged as belonging to us.
	return r.resourceClient.Delete(ctx, obj.GetName())
}

// Reconcile allows the TeleportResourceReconciler to implement the reconcile.Reconciler interface
func (r TeleportResourceReconciler[T, K]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	kubeResource := newKubeResource[T, K]()
	return r.Do(ctx, req, kubeResource)
}

// SetupWithManager have a controllerruntime.Manager run the TeleportResourceReconciler
func (r TeleportResourceReconciler[T, K]) SetupWithManager(mgr ctrl.Manager) error {
	kubeResource := newKubeResource[T, K]()
	return ctrl.
		NewControllerManagedBy(mgr).
		For(kubeResource).
		WithEventFilter(
			buildPredicate(),
		).
		Complete(r)
}

// newKubeResource creates a new TeleportKubernetesResource
// the function supports structs or pointer to struct implementations of the TeleportKubernetesResource interface
func newKubeResource[T TeleportResource, K TeleportKubernetesResource[T]]() K {
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
// - the resource generation changes
// - the resource finalizers change
// - the resource annotations change
// - the resource labels change
// - the resource is created
// - the resource is deleted
// It does not trigger the reconciliation when:
// - the resource status changes
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
