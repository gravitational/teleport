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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
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

func (r ResourceBaseReconciler) Do(ctx context.Context, req ctrl.Request, obj kclient.Object) (ctrl.Result, error) {
	// https://sdk.operatorframework.io/docs/building-operators/golang/advanced-topics/#external-resources
	log := log.FromContext(ctx).WithValues("namespacedname", req.NamespacedName)

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
			if err := r.DeleteExternal(ctx, obj); err != nil {
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

		if err := r.Update(ctx, obj); err != nil {
			return ctrl.Result{}, trace.Wrap(err, "failed to add finalizer")
		}
		return ctrl.Result{}, nil
	}

	// Create or update
	log.Info("upsert object in Teleport")
	if err := r.UpsertExternal(ctx, obj); err != nil {
		return ctrl.Result{}, trace.Wrap(err)
	}

	return ctrl.Result{}, nil
}

func hasOriginLabel(obj kclient.Object) bool {
	if obj.GetLabels() == nil {
		return false
	}

	for k := range obj.GetLabels() {
		if k == types.OriginLabel {
			return true
		}
	}

	return false
}

func addOriginLabel(ctx context.Context, k8sClient kclient.Client, obj kclient.Object) error {
	k8sObjLabels := obj.GetLabels()
	if k8sObjLabels == nil {
		k8sObjLabels = make(map[string]string)
	}

	k8sObjLabels[types.OriginLabel] = types.OriginKubernetes
	obj.SetLabels(k8sObjLabels)

	return trace.Wrap(k8sClient.Update(ctx, obj))
}
