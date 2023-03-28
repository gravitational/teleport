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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv2 "github.com/gravitational/teleport/operator/apis/resources/v2"
	"github.com/gravitational/teleport/operator/sidecar"
)

// TODO: Have the User controller to use the generic Teleport reconciler

// UserReconciler reconciles a TeleportUser object
type UserReconciler struct {
	kclient.Client
	Scheme                 *runtime.Scheme
	TeleportClientAccessor sidecar.ClientAccessor
}

//+kubebuilder:rbac:groups=resources.teleport.dev,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=resources.teleport.dev,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=resources.teleport.dev,resources=users/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ResourceBaseReconciler{
		Client:         r.Client,
		DeleteExternal: r.Delete,
		UpsertExternal: r.Upsert,
	}.Do(ctx, req, &resourcesv2.TeleportUser{})
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resourcesv2.TeleportUser{}).
		Complete(r)
}

func (r *UserReconciler) Delete(ctx context.Context, obj kclient.Object) error {
	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return teleportClient.DeleteUser(ctx, obj.GetName())
}

func (r *UserReconciler) Upsert(ctx context.Context, obj kclient.Object) error {
	k8sResource, ok := obj.(*resourcesv2.TeleportUser)
	if !ok {
		return fmt.Errorf("failed to convert Object into resource object: %T", obj)
	}
	teleportResource := k8sResource.ToTeleport()

	teleportClient, err := r.TeleportClientAccessor(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	existingResource, err := teleportClient.GetUser(teleportResource.GetName(), false)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	exists := !trace.IsNotFound(err)

	if exists {
		newOwnershipCondition, isOwned := checkOwnership(existingResource)
		meta.SetStatusCondition(&k8sResource.Status.Conditions, newOwnershipCondition)
		if !isOwned {
			silentUpdateStatus(ctx, r.Client, k8sResource)
			return trace.AlreadyExists("unowned resource '%s' already exists", existingResource.GetName())
		}
	} else {
		// The resource does not yet exist
		meta.SetStatusCondition(&k8sResource.Status.Conditions, newResourceCondition)
	}

	r.AddTeleportResourceOrigin(teleportResource)

	if !exists {
		err = teleportClient.CreateUser(ctx, teleportResource)
	} else {
		// We don't want to lose the "created_by" data populated on creation
		teleportResource.SetCreatedBy(existingResource.GetCreatedBy())
		err = teleportClient.UpdateUser(ctx, teleportResource)
	}
	// If an error happens we want to put it in status.conditions before returning.
	newReconciliationCondition := getReconciliationConditionFromError(err)
	meta.SetStatusCondition(&k8sResource.Status.Conditions, newReconciliationCondition)
	if err != nil {
		silentUpdateStatus(ctx, r.Client, k8sResource)
		return trace.Wrap(err)
	}

	// We update the status conditions on exit
	return trace.Wrap(r.Status().Update(ctx, k8sResource))
}

func (r *UserReconciler) AddTeleportResourceOrigin(resource types.User) {
	metadata := resource.GetMetadata()
	if metadata.Labels == nil {
		metadata.Labels = make(map[string]string)
	}
	metadata.Labels[types.OriginLabel] = types.OriginKubernetes
	resource.SetMetadata(metadata)
}
