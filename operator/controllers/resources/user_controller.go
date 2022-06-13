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
	resourcesv2 "github.com/gravitational/teleport/operator/apis/resources/v2"
	"github.com/gravitational/trace"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	kclient.Client
	Scheme         *runtime.Scheme
	TeleportClient *client.Client
}

//+kubebuilder:rbac:groups=resources.teleport.dev,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=resources.teleport.dev,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=resources.teleport.dev,resources=users/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the User object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ResourceBaseReconciler{
		Client:         r.Client,
		DeleteExternal: r.Delete,
		UpsertExternal: r.Upsert,
	}.Do(ctx, req, &resourcesv2.User{})
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resourcesv2.User{}).
		Complete(r)
}

func (r *UserReconciler) Delete(ctx context.Context, obj kclient.Object) error {
	return r.TeleportClient.DeleteUser(ctx, obj.GetName())
}

func (r *UserReconciler) Upsert(ctx context.Context, obj kclient.Object) error {
	k8sResource, ok := obj.(*resourcesv2.User)
	if !ok {
		return fmt.Errorf("failed to convert Object into resource object: %T", obj)
	}
	teleportResource := k8sResource.ToTeleport()

	_, err := r.TeleportClient.GetUser(teleportResource.GetName(), false)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		if !hasOriginLabel(obj) {
			return addOriginLabel(ctx, r.Client, obj)
		}

		return trace.Wrap(r.TeleportClient.CreateUser(ctx, teleportResource))
	}

	return trace.Wrap(r.TeleportClient.UpdateUser(ctx, teleportResource))
}
