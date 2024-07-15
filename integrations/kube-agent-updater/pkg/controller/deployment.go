/*
Copyright 2023 Gravitational, Inc.

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

package controller

import (
	"context"
	"errors"

	"github.com/gravitational/trace"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

// DeploymentVersionUpdater Reconciles a podSpec by changing its image
type DeploymentVersionUpdater struct {
	VersionUpdater
	kclient.Client
	Scheme *runtime.Scheme
}

// Reconcile treats a reconciliation request for a Deployment object. It gets the
// object, retrieves its current version, and calls the VersionUpdater to find
// if the Deployment should be updated. If it's the case, it changes the
// Teleport image version and updates the Deployment in Kubernetes.
func (r *DeploymentVersionUpdater) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithValues("namespacedname", req.NamespacedName, "kind", "Deployment")
	// We set the logger and a max timout for the whole reconciliation loop
	// This protects us from an external call stalling the reconciliation loop.
	ctx, cancel := context.WithTimeout(ctrllog.IntoContext(ctx, log), reconciliationTimeout)
	defer cancel()

	// Get the object we are reconciling
	var obj appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, trace.Wrap(err)
	}
	if skipReconciliation(&obj) {
		log.Info("Reconciliation disabled by resource annotations. Skipping.")
		return requeueLater, nil
	}

	// Get the current and past version
	currentVersion, err := getWorkloadVersion(obj.Spec.Template.Spec)
	if err != nil {
		switch trace.Unwrap(err).(type) {
		case *trace.BadParameterError:
			log.Info("Teleport container found, but failed to get version from the img tag. Will continue and do a version update.")
		default:
			log.Error(err, "Unexpected error, not updating.")
			return requeueLater, nil
		}
	}

	image, err := r.GetVersion(ctx, &obj, currentVersion)
	var (
		noNewVersionErr *version.NoNewVersionError
		maintenanceErr  *MaintenanceNotTriggeredError
		trustErr        *trace.TrustError
	)
	switch {
	case errors.As(err, &noNewVersionErr):
		// Error contains the detected versions
		log.Info("Deployment is already up-to-date, not updating.", "err", err)
		return requeueLater, nil
	case errors.As(err, &maintenanceErr):
		// Not logging the error because it provides no other information than its type.
		log.Info("No maintenance triggered, not updating.", "currentVersion", currentVersion)
		return requeueLater, nil
	case errors.As(err, &trustErr):
		// Logging as error as image verification should not fail under normal use
		log.Error(trustErr.OrigError(), "Image verification failed, not updating.")
		return requeueLater, nil
	case err != nil:
		log.Error(err, "Unexpected error, not updating.")
		return requeueLater, nil
	}

	log.Info("Updating podSpec with image", "image", image.String())
	err = setContainerImageFromPodSpec(&obj.Spec.Template.Spec, teleportContainerName, image.String())
	if err != nil {
		log.Error(err, "Unexpected error, not updating.")
		return requeueLater, nil
	}

	if err = r.Update(ctx, &obj); err != nil {
		log.Error(err, "Unexpected error, not updating.")
		return requeueNow, nil
	}
	return requeueLater, nil
}

// SetupWithManager makes the DeploymentVersionUpdater managed by a ctrl.Manager.
// Once started, the manager will send Deployment reconciliation requests to the
// DeploymentVersionUpdater controller.
func (r *DeploymentVersionUpdater) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Complete(r)
}
