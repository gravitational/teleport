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

	"github.com/docker/distribution/reference"
	"github.com/gravitational/trace"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/version"
)

// DeploymentVersionUpdater Reconciles a deployment by changing its image
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
	log := ctrllog.FromContext(ctx).WithValues("namespacedname", req.NamespacedName)
	ctx = ctrllog.IntoContext(ctx, log)

	// Get the object we are reconciling
	var obj appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, trace.Wrap(err)
	}

	// Get the current and past version
	currentVersion, err := getDeploymentVersion(&obj)
	if err != nil {
		switch trace.Unwrap(err).(type) {
		case *trace.BadParameterError:
			log.Info("Teleport container found, but failed to get version from the img tag. Will continue and do a version update.")
		default:
			return requeueLater, trace.Wrap(err)
		}
	}

	image, err := r.GetVersion(ctx, &obj, currentVersion)
	var (
		noNewVersionErr *NoNewVersionError
		maintenanceErr  *MaintenanceNotTriggeredError
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
	case trace.IsTrustError(err):
		// Logging as error as image verification should not fail under normal use
		log.Error(err, "Image verification failed, not updating.")
		return requeueLater, trace.Wrap(err)
	case err != nil:
		log.Error(err, "Unexpected error, not updating.")
		return requeueLater, trace.Wrap(err)
	}

	log.Info("Updating deployment with image", "image", image.String())
	err = setContainerImageFromPodSpec(&obj.Spec.Template.Spec, teleportContainerName, image.String())
	if err != nil {
		return requeueLater, trace.Wrap(err)
	}

	if err = r.Update(ctx, &obj); err != nil {
		return requeueNow, trace.Wrap(err)
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

func getDeploymentVersion(deployment *appsv1.Deployment) (string, error) {
	var current string
	image, err := getContainerImageFromPodSpec(deployment.Spec.Template.Spec, teleportContainerName)
	if err != nil {
		return current, trace.Wrap(err)
	}

	// TODO: put this in a function and reuse for statefulset
	imageRef, err := reference.ParseNamed(image)
	if err != nil {
		return current, trace.Wrap(err)
	}
	taggedImageRef, ok := imageRef.(reference.Tagged)
	if !ok {
		return "", trace.BadParameter("imageRef %s is not tagged", imageRef)
	}
	current = taggedImageRef.Tag()
	current, err = version.EnsureSemver(current)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return current, nil
}
