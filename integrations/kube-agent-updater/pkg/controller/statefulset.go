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

package controller

import (
	"context"
	"errors"
	"sync"

	"github.com/gravitational/trace"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/podutils"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

type StatefulSetVersionUpdater struct {
	VersionUpdater
	kclient.Client
	Scheme *runtime.Scheme
}

// Reconcile treats a reconciliation request for a StatefulSet object. It gets the
// object, retrieves its current version, and calls the VersionUpdater to find
// if the StatefulSet should be updated. If it's the case, it changes the
// Teleport image version and updates the StatefulSet in Kubernetes.
//
// WARNING: StatefulSets rollouts can end up being stuck because of unready pods.
// We must delete those unhealthy pods to ensure the rollout is not blocked.
// Deleting only after an update is not idempotent, and deleting every
// reconciliation or maintenance might be disruptive and cause misleading error
// if something else is broken (i.e. the state is invalid but we keep deleting
// the pods). To mitigate the disruption, we only delete unhealthy pods whose
// spec is not based on the current PodTemplate.
// We attempt to unstuck a rollout:
//   - when a maintenance was triggered but no new version was found (unhealthy
//     pods will trigger maintenance thanks to the WorkloadUnhealthyTrigger)
//   - when a maintenance was triggered, new version was found, but we failed to
//     validate the image.
//   - when the version was successfully updated
//
// We do not try to unblock in the following cases:
//   - when a maintenance was not triggered (nothing to do, the most common case)
//   - when we encounter an unknown error when checking maintenance,version,image
//   - when we face an error when updating the statefulset (99% chance we
//     conflicted with something else and are being requeued, the update will
//     pass the next time)
func (r *StatefulSetVersionUpdater) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithValues("namespacedname", req.NamespacedName, "kind", "StatefulSet")
	// We set the logger and a max timout for the whole reconciliation loop
	// This protects us from an external call stalling the reconciliation loop.
	ctx, cancel := context.WithTimeout(ctrllog.IntoContext(ctx, log), reconciliationTimeout)
	defer cancel()

	// Get the object we are reconciling
	var obj appsv1.StatefulSet
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
		switch {
		case trace.IsBadParameter(err):
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
		log.Info("StatefulSet is already up-to-date, not updating.", "err", err)
		if err := r.unblockStatefulSetRolloutIfStuck(ctx, &obj); err != nil {
			log.Error(err, "statefulset unblocking failed, the rollout might get stuck")
		}
		return requeueLater, nil
	case errors.As(err, &maintenanceErr):
		// Not logging the error because it provides no other information than its type.
		log.Info("No maintenance triggered, not updating.", "currentVersion", currentVersion)
		// No need to check for blocked rollout because the unhealthy workload
		// trigger has not approved the maintenance
		return requeueLater, nil
	case errors.As(err, &trustErr):
		// Logging as error as image verification should not fail under normal use
		log.Error(trustErr.OrigError(), "Image verification failed, not updating.")
		if err := r.unblockStatefulSetRolloutIfStuck(ctx, &obj); err != nil {
			log.Error(err, "statefulset unblocking failed, the rollout might get stuck")
		}
		return requeueLater, nil
	case err != nil:
		log.Error(err, "Unexpected error, not updating.")
		// Not trying to unblock a stuck rollout because unknown error typically
		// lead to infinite reconciliations, we don't want to DoS the apiserver
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
	if err := r.unblockStatefulSetRolloutIfStuck(ctx, &obj); err != nil {
		log.Error(err, "statefulset unblocking failed, the rollout might get stuck")
	}
	return requeueLater, nil
}

// SetupWithManager makes the DeploymentVersionUpdater managed by a ctrl.Manager.
// Once started, the manager will send Deployment reconciliation requests to the
// DeploymentVersionUpdater controller.
func (r *StatefulSetVersionUpdater) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.StatefulSet{}).
		Complete(r)
}

// unblockStatefulSetRolloutIfStuck looks for old unhealthy pods belonging to
// the StatefulSet that might block a rollout and deletes them.
// Note: this might be called often, we might decide to enable cache for pods
// for performance purposes (we trade more memory usage on the updater side,
// against less latency and calls to the apiserver)
func (r *StatefulSetVersionUpdater) unblockStatefulSetRolloutIfStuck(ctx context.Context, sts *appsv1.StatefulSet) error {
	log := ctrllog.FromContext(ctx)
	verboseLog := ctrllog.FromContext(ctx).V(1)

	if sts.Status.UpdateRevision == "" {
		return trace.BadParameter("statefulset '%s' UpdateRevision empty", sts.Name)
	}

	// First we get all pods managed by the StatefulSet
	stsLabelSelector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
	if err != nil {
		return trace.Wrap(err)
	}

	var managedPodsList v1.PodList
	listOptions := []kclient.ListOption{kclient.InNamespace(sts.Namespace), kclient.MatchingLabelsSelector{Selector: stsLabelSelector}}
	listCtx, cancel := context.WithTimeout(ctx, kubeClientTimeout)
	defer cancel()
	err = r.List(listCtx, &managedPodsList, listOptions...)
	if err != nil {
		return trace.Wrap(err)
	}

	// *v1.Pod implements metav1.Object, it's easier to convert now from
	// []v1.Pod to []*v1.Pod. e.g. this is required for listNames()
	managedPods := podutils.PodListToListOfPods(&managedPodsList)
	verboseLog.Info("statefulset managed pods", "managedPodsList", podutils.ListNames(managedPods))

	// Then we filter out all malformed pods, healthy pods, and pods belonging
	// to the latest controller revision
	filters := podutils.Filters{
		podutils.MustHaveControllerRevisionLabel,
		podutils.IsUnhealthy,
		podutils.Not(podutils.BelongsControllerRevisionFilter(sts.Status.UpdateRevision)),
	}
	oldUnhealthyPods := filters.Apply(ctx, managedPods)

	if len(oldUnhealthyPods) == 0 {
		verboseLog.Info(" no statefulset unhealthy pods from old revisions")
		return nil
	}
	log.Info(
		"unhealthy pods from an old revision found, deleting them to unblock potential StatefulSet rollout",
		"oldUnhealthyPods", podutils.ListNames(oldUnhealthyPods),
	)

	// Finally, we delete the pods that might block the pod rollout. The
	// StatefulSet controller will issue new pods based on the latest
	// spec/controller revision.

	// We cannot use the DELETECOLLECTION method here because it is invoked with
	// a selector and not a pod list. It's not possible to express our
	// healthy/unhealthy definition with selectors, so we delete them individually.
	errs := make(chan error, len(oldUnhealthyPods))
	var wg sync.WaitGroup
	deleteCtx, cancel := context.WithTimeout(ctx, kubeClientTimeout)
	defer cancel()

	for _, pod := range oldUnhealthyPods {
		wg.Add(1)
		pod := pod
		go func() {
			defer wg.Done()
			errs <- r.Delete(deleteCtx, pod)
		}()
	}

	wg.Wait()
	// Closing should be safe now as all senders are stopped
	close(errs)
	return trace.NewAggregateFromChannel(errs, context.Background())
}
