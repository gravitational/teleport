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

	"github.com/gravitational/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// DeletionFinalizer is a name of finalizer added to resource's 'finalizers' field
// for tracking deletion events.
const DeletionFinalizer = "resources.teleport.dev/deletion"

type DeleteExternal func(context.Context, kclient.Object) error
type UpsertExternal func(context.Context, kclient.Object) error

type ResourceBaseReconciler struct {
	kclient.Client
	DeleteExternal DeleteExternal
	UpsertExternal UpsertExternal
}

/*
Do will receive an update request and reconcile the resource.

When an event arrives we must propagate that change into the Teleport cluster.
We have two types of events: update/create and delete.

For creating/updating we check if the resource exists in Teleport
- if it does, we update it
- otherwise we create it
Always using the state of the resource in the cluster as the source of truth.

For deleting, the recommendation is to use finalizers.
Finalizers allow us to map an external resource to a kubernetes resource.
So, when we create or update a resource, we add our own finalizer to the kubernetes resource list of finalizers.

For a delete event which has our finalizer: the resource is deleted in Teleport.
If it doesn't have the finalizer, we do nothing.

----

Every time we update a resource in Kubernetes (adding finalizers or the OriginLabel), we end the reconciliation process.
Afterwards, we receive the request again and we progress to the next step.
This allow us to progress with smaller changes and avoid a long-running reconciliation.
*/
func (r ResourceBaseReconciler) Do(ctx context.Context, req ctrl.Request, obj kclient.Object) (ctrl.Result, error) {
	// https://sdk.operatorframework.io/docs/building-operators/golang/advanced-topics/#external-resources
	log := ctrllog.FromContext(ctx).WithValues("namespacedname", req.NamespacedName)

	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("not found")
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get resource")
		return ctrl.Result{}, trace.Wrap(err)
	}

	hasDeletionFinalizer := controllerutil.ContainsFinalizer(obj, DeletionFinalizer)
	isMarkedToBeDeleted := !obj.GetDeletionTimestamp().IsZero()

	// Delete
	if isMarkedToBeDeleted {
		if hasDeletionFinalizer {
			log.Info("deleting object in Teleport")
			if err := r.DeleteExternal(ctx, obj); err != nil && !trace.IsNotFound(err) {
				return ctrl.Result{}, trace.Wrap(err)
			}

			log.Info("removing finalizer")
			controllerutil.RemoveFinalizer(obj, DeletionFinalizer)
			if err := r.Update(ctx, obj); err != nil {
				return ctrl.Result{}, trace.Wrap(err, "failed to remove finalizer after deleting in teleport")
			}
		}

		// marked to be deleted without finalizer
		return ctrl.Result{}, nil
	}

	if !hasDeletionFinalizer {
		log.Info("adding finalizer")
		controllerutil.AddFinalizer(obj, DeletionFinalizer)

		err := r.Update(ctx, obj)

		return ctrl.Result{}, trace.Wrap(err, "failed to add finalizer")
	}

	// Create or update
	log.Info("upsert object in Teleport")
	err := r.UpsertExternal(ctx, obj)
	return ctrl.Result{}, trace.Wrap(err)
}
