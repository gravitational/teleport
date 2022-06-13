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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	resourcesv5 "github.com/gravitational/teleport/operator/apis/resources/v5"
	"github.com/gravitational/trace"
)

// RoleReconciler reconciles a Role object
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
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Role object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *RoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ResourceBaseReconciler{
		Client:         r.Client,
		DeleteExternal: r.Delete,
		UpsertExternal: r.Upsert,
	}.Do(ctx, req, &resourcesv5.Role{})
}

// SetupWithManager sets up the controller with the Manager.
func (r *RoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resourcesv5.Role{}).
		Complete(r)
}

func (r *RoleReconciler) Delete(ctx context.Context, obj kclient.Object) error {
	return r.TeleportClient.DeleteRole(ctx, obj.GetName())
}

func (r *RoleReconciler) Upsert(ctx context.Context, obj kclient.Object) error {
	k8sResource, ok := obj.(*resourcesv5.Role)
	if !ok {
		return fmt.Errorf("failed to convert Object into resource object: %T", obj)
	}
	teleportResource := k8sResource.ToTeleport()

	_, err := r.TeleportClient.GetRole(ctx, teleportResource.GetName())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		if !hasOriginLabel(obj) {
			return addOriginLabel(ctx, r.Client, obj)
		}
	}
	return r.TeleportClient.UpsertRole(ctx, teleportResource)
}
