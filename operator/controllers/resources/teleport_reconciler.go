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

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
)

type TeleportResource interface {
	GetName() string
	SetOrigin(string)
	GetMetadata() types.Metadata
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
type TeleportResourceClient[T TeleportResource] interface {
	Get(context.Context, string) (T, error)
	Create(context.Context, T) error
	Update(context.Context, T) error
	Delete(context.Context, string) error
}

// NewTeleportResourceReconciler instanciates a TeleportResourceReconciler from a TeleportResourceClient.
func NewTeleportResourceReconciler[T TeleportResource, K TeleportKubernetesResource[T]](
	client kclient.Client,
	resourceClient TeleportResourceClient[T]) *TeleportResourceReconciler[T, K] {

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
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// If err is nil, we found the resource. If err != nil (and we did return), then the error was `NotFound`
	exists := err == nil

	if exists {
		newOwnershipCondition, isOwned := checkOwnership(existingResource)
		meta.SetStatusCondition(k8sResource.StatusConditions(), newOwnershipCondition)
		if !isOwned {
			return trace.NewAggregate(
				trace.AlreadyExists("unowned resource '%s' already exists", existingResource.GetName()),
				r.Status().Update(ctx, k8sResource),
			)
		}
	} else {
		meta.SetStatusCondition(k8sResource.StatusConditions(), newResourceCondition)
	}

	teleportResource.SetOrigin(types.OriginKubernetes)

	if !exists {
		err = r.resourceClient.Create(ctx, teleportResource)
	} else {
		/* TODO: handle modifier logic like CreatedBy for users,
		we can add mutate logic, diffing could also happen here */
		err = r.resourceClient.Update(ctx, teleportResource)
	}
	// If an error happens we want to put it in status.conditions before returning.
	newReconciliationCondition := getReconciliationConditionFromError(err)
	meta.SetStatusCondition(k8sResource.StatusConditions(), newReconciliationCondition)
	if err != nil {
		return trace.NewAggregate(err, r.Status().Update(ctx, k8sResource))
	}

	// We update the status conditions on exit
	return trace.Wrap(r.Status().Update(ctx, k8sResource))
}

// Delete is the TeleportResourceReconciler of the ResourceBaseReconciler DeleteExertal
func (r TeleportResourceReconciler[T, K]) Delete(ctx context.Context, obj kclient.Object) error {
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
	return ctrl.NewControllerManagedBy(mgr).For(kubeResource).Complete(r)
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
