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

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	v5 "github.com/gravitational/teleport/integrations/operator/apis/resources/v5"
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
	Scheme         *runtime.Scheme
	TeleportClient *client.Client
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
	obj, err := GetUnstructuredObjectFromGVK(TeleportRoleGVKV5)
	if err != nil {
		return ctrl.Result{}, trace.Wrap(err, "creating object in which the CR will be unmarshalled")
	}
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
	obj, err := GetUnstructuredObjectFromGVK(TeleportRoleGVKV5)
	if err != nil {
		return trace.Wrap(err, "creating the model object for the manager watcher/client")
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(obj).
		WithEventFilter(buildPredicate()).
		Complete(r)
}

func (r *RoleReconciler) Delete(ctx context.Context, obj kclient.Object) error {
	return r.TeleportClient.DeleteRole(ctx, obj.GetName())
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
	existingResource, err := r.TeleportClient.GetRole(ctx, teleportResource.GetName())
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

	if existingResource != nil {
		teleportResource.SetRevision(existingResource.GetRevision())
	}
	r.AddTeleportResourceOrigin(teleportResource)

	// If an error happens we want to put it in status.conditions before returning.
	_, err = r.TeleportClient.UpsertRole(ctx, teleportResource)
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

func GetUnstructuredObjectFromGVK(gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
	if gvk.Empty() {
		return nil, trace.BadParameter("cannot create an object for an empty GVK, aborting")
	}
	obj := unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	return &obj, nil
}

func NewRoleReconciler(client kclient.Client, tClient *client.Client) (Reconciler, error) {
	return &RoleReconciler{
		Client:         client,
		Scheme:         Scheme,
		TeleportClient: tClient,
	}, nil
}
