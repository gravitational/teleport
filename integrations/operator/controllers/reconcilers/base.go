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

	"github.com/gravitational/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// TODO(hugoShaka) : merge the base reconciler with the generic reocnciler.
// This was a separate struct for backward compatibility but we removed the last
// controller relying directly on the base reconciler.

const (
	// DeletionFinalizer is a name of finalizer added to Resource's 'finalizers' field
	// for tracking deletion events.
	DeletionFinalizer = "resources.teleport.dev/deletion"
	// AnnotationFlagIgnore is the Kubernetes annotation containing the "ignore" flag.
	// When set to true, the operator will not reconcile the CR.
	AnnotationFlagIgnore = "teleport.dev/ignore"
	// AnnotationFlagKeep is the Kubernetes annotation containing the "keep" flag.
	// When set to true, the operator will not delete the Teleport Resource if the
	// CR is deleted.
	AnnotationFlagKeep = "teleport.dev/keep"
)

type DeleteExternal func(context.Context, kclient.Object) error
type UpsertExternal func(context.Context, kclient.Object) error

type ResourceBaseReconciler struct {
	kclient.Client
	DeleteExternal DeleteExternal
	UpsertExternal UpsertExternal
}

/*
Do will receive an update request and reconcile the Resource.

When an event arrives we must propagate that change into the Teleport cluster.
We have two types of events: update/create and delete.

For creating/updating we check if the Resource exists in Teleport
- if it does, we update it
- otherwise we create it
Always using the state of the Resource in the cluster as the source of truth.

For deleting, the recommendation is to use finalizers.
Finalizers allow us to map an external Resource to a kubernetes Resource.
So, when we create or update a Resource, we add our own finalizer to the kubernetes Resource list of finalizers.

For a delete event which has our finalizer: the Resource is deleted in Teleport.
If it doesn't have the finalizer, we do nothing.

----

Every time we update a Resource in Kubernetes (adding finalizers or the OriginLabel), we end the reconciliation process.
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
		log.Error(err, "failed to get Resource")
		return ctrl.Result{}, trace.Wrap(err)
	}

	if isIgnored(obj) {
		log.Info(fmt.Sprintf("Resource is flagged with annotation %q, it will not be reconciled.", AnnotationFlagIgnore))
		return ctrl.Result{}, nil
	}

	hasDeletionFinalizer := controllerutil.ContainsFinalizer(obj, DeletionFinalizer)
	isMarkedToBeDeleted := !obj.GetDeletionTimestamp().IsZero()

	// Delete
	if isMarkedToBeDeleted {
		if hasDeletionFinalizer {
			if isKept(obj) {
				log.Info(fmt.Sprintf("Resource is flagged with annotation %q, it will not be deleted in Teleport.", AnnotationFlagKeep))
			} else {
				log.Info("deleting object in Teleport")
				if err := r.DeleteExternal(ctx, obj); err != nil && !trace.IsNotFound(err) {
					return ctrl.Result{}, trace.Wrap(err)
				}
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

// isIgnored checks if the CR should be ignored
func isIgnored(obj kclient.Object) bool {
	return checkAnnotationFlag(obj, AnnotationFlagIgnore, false /* defaults to false */)
}

// isKept checks if the Teleport Resource should be kept if the CR is deleted
func isKept(obj kclient.Object) bool {
	return checkAnnotationFlag(obj, AnnotationFlagKeep, false /* defaults to false */)
}
