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
	"fmt"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	v5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
	"github.com/gravitational/teleport/integrations/operator/sidecar"
)

const teleportRoleKind = "TeleportRole"

// TODO(for v12): Have the Role controller to use the generic Teleport reconciler
// This means we'll have to move back to a statically typed client.
// This will require removing the crdgen hack, fixing TeleportRole JSON serialization

var TeleportRoleGVKV5 = schema.GroupVersionKind{
	Group:   v5.GroupVersion.Group,
	Version: v5.GroupVersion.Version,
	Kind:    teleportRoleKind,
}

// RoleReconciler reconciles a TeleportRole object
type RoleReconciler struct {
	kclient.Client
	Scheme                 *runtime.Scheme
	TeleportClientAccessor sidecar.ClientAccessor
}

//+kubebuilder:rbac:groups=resources.teleport.dev,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=resources.teleport.dev,resources=roles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=resources.teleport.dev,resources=roles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *RoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// The TeleportRole OpenAPI spec does not validate typing of Label fields like `node_labels`.
	// This means we can receive invalid data, by default it won't be unmarshalled properly and will crash the operator.
	// To handle this more gracefully we unmarshall first in an unstructured object.
	// The unstructured object will be converted later to a typed one, in r.UpsertExternal.
	// See `/operator/crdgen/schemagen.go` and https://github.com/gravitational/teleport/issues/15204 for context.
	// TODO: (Check how to handle multiple versions)
	obj := GetUnstructuredObjectFromGVK(TeleportRoleGVKV5)
	return ResourceBaseReconciler{
		Client:         r.Client,
		DeleteExternal: r.Delete,
		UpsertExternal: r.Upsert,
	}.Do(ctx, req, obj)
}

// SetupWithManager sets up the controller with the Manager.
func (r *RoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// The TeleportRole OpenAPI spec does not validate typing of Label fields like `node_labels`.
	// This means we can receive invalid data, by default it won't be unmarshalled properly and will crash the operator
	// To handle this more gracefully we unmarshall first in an unstructured object.
	// The unstructured object will be converted later to a typed one, in r.UpsertExternal.
	// See `/operator/crdgen/schemagen.go` and https://github.com/gravitational/teleport/issues/15204 for context
	// TODO: (Check how to handle multiple versions)
	obj := GetUnstructuredObjectFromGVK(TeleportRoleGVKV5)
	return ctrl.NewControllerManagedBy(mgr).
		For(obj).
		WithEventFilter(buildPredicate()).
		Complete(r)
}

func (r *RoleReconciler) Delete(ctx context.Context, obj kclient.Object) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return teleportClient.DeleteRole(ctx, obj.GetName())
}

func (r *RoleReconciler) Upsert(ctx context.Context, obj kclient.Object) error {
	// We receive an unstructured object. We convert it to a typed TeleportRole object and gracefully handle errors.
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("failed to convert Object into resource object: %T", obj)
	}
	k8sResource := &v5.TeleportRole{}

	// If an error happens we want to put it in status.conditions before returning.
	err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(
		u.Object,
		k8sResource, true, /* returnUnknownFields */
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

	// Converting the Kubernetes resource into a Teleport one, checking potential ownership issues.
	teleportResource := k8sResource.ToTeleport()
	teleportClient, err := r.TeleportClientAccessor(ctx)
	updateErr = updateStatus(updateStatusConfig{
		ctx:         ctx,
		client:      r.Client,
		k8sResource: k8sResource,
		condition:   getTeleportClientConditionFromError(err),
	})
	if err != nil || updateErr != nil {
		return trace.NewAggregate(err, updateErr)
	}

	existingResource, err := teleportClient.GetRole(ctx, teleportResource.GetName())
	updateErr = updateStatus(updateStatusConfig{
		ctx:         ctx,
		client:      r.Client,
		k8sResource: k8sResource,
		condition:   getReconciliationConditionFromError(err, true /* ignoreNotFound */),
	})
	if err != nil && !trace.IsNotFound(err) || updateErr != nil {
		return trace.NewAggregate(err, updateErr)
	}

	if err == nil {
		// The resource already exists
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
		// The resource does not yet exist
		if updateErr = updateStatus(updateStatusConfig{
			ctx:         ctx,
			client:      r.Client,
			k8sResource: k8sResource,
			condition:   newResourceCondition,
		}); updateErr != nil {
			return trace.Wrap(updateErr)
		}
	}

	r.AddTeleportResourceOrigin(teleportResource)

	// If an error happens we want to put it in status.conditions before returning.
	_, err = teleportClient.UpsertRole(ctx, teleportResource)
	updateErr = updateStatus(updateStatusConfig{
		ctx:         ctx,
		client:      r.Client,
		k8sResource: k8sResource,
		condition:   getReconciliationConditionFromError(err, false /* ignoreNotFound */),
	})
	// We update the status conditions on exit
	return trace.NewAggregate(err, updateErr)
}

func (r *RoleReconciler) AddTeleportResourceOrigin(resource types.Role) {
	metadata := resource.GetMetadata()
	if metadata.Labels == nil {
		metadata.Labels = make(map[string]string)
	}
	metadata.Labels[types.OriginLabel] = types.OriginKubernetes
	resource.SetMetadata(metadata)
}

func GetUnstructuredObjectFromGVK(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	return &obj
}
