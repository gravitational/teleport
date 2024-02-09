/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package resources

import (
	"context"
	"fmt"
	"reflect"
	"slices"

	"github.com/gravitational/trace"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	gvk            schema.GroupVersionKind
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
) (*TeleportResourceReconciler[T, K], error) {
	gvk, err := gvkFromScheme[T, K](Scheme)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reconciler := &TeleportResourceReconciler[T, K]{
		ResourceBaseReconciler: ResourceBaseReconciler{Client: client},
		resourceClient:         resourceClient,
		gvk:                    gvk,
	}
	reconciler.ResourceBaseReconciler.UpsertExternal = reconciler.Upsert
	reconciler.ResourceBaseReconciler.DeleteExternal = reconciler.Delete
	return reconciler, nil
}

// Upsert is the TeleportResourceReconciler of the ResourceBaseReconciler UpsertExternal
// It contains the logic to check if the resource already exists, if it is owned by the operator and what
// to do to reconcile the Teleport resource based on the Kubernetes one.
func (r TeleportResourceReconciler[T, K]) Upsert(ctx context.Context, obj kclient.Object) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("failed to convert Object into resource object: %T", obj)
	}
	k8sResource := newKubeResource[T, K]()

	// If an error happens we want to put it in status.conditions before returning.
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

	existingResource, err := r.resourceClient.Get(ctx, teleportResource.GetName())
	updateErr = updateStatus(updateStatusConfig{
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
	return r.resourceClient.Delete(ctx, obj.GetName())
}

// Reconcile allows the TeleportResourceReconciler to implement the reconcile.Reconciler interface
func (r TeleportResourceReconciler[T, K]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	obj, err := GetUnstructuredObjectFromGVK(r.gvk)
	if err != nil {
		return ctrl.Result{}, trace.Wrap(err, "creating object in which the CR will be unmarshalled")
	}
	return r.Do(ctx, req, obj)
}

// SetupWithManager have a controllerruntime.Manager run the TeleportResourceReconciler
func (r TeleportResourceReconciler[T, K]) SetupWithManager(mgr ctrl.Manager) error {
	// The TeleportResourceReconciler uses unstructured objects because of a silly json marshaling
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

// gvkFromScheme looks up the GVK from the a runtime scheme.
// The structured type must have been registered before in the scheme. This function is used when you have a structured
// type, a scheme containing this structured type, and want to build an unstructured object for the same GVK.
func gvkFromScheme[T TeleportResource, K TeleportKubernetesResource[T]](scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
	structuredObj := newKubeResource[T, K]()
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
