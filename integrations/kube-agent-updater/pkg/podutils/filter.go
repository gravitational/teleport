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

package podutils

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// FilterFunc takes a pod and checks if it matches a specific criteria
// For example: "is the pod healthy?", "does the pod have this label?"
// It returns true if the pod meets the criteria
type FilterFunc func(ctx context.Context, pod *v1.Pod) bool

// Filters is a list of FilterFunc.
type Filters []FilterFunc

// Apply filters a pod list with all Filters and return a list of the pods
// that matched against all Filters.
func (f Filters) Apply(ctx context.Context, pods []*v1.Pod) []*v1.Pod {
	var filteredPods []*v1.Pod
	for _, pod := range pods {
		if f.podMatch(ctx, pod) {
			filteredPods = append(filteredPods, pod)
		}
	}
	return filteredPods
}

// podMatch evaluates if a single pod matches against all Filters.
func (f Filters) podMatch(ctx context.Context, pod *v1.Pod) bool {
	for _, filter := range f {
		if !filter(ctx, pod) {
			return false
		}
	}
	return true
}

// Not takes a FilterFunc and builds the opposite FilterFunc.
func Not(filterFunc FilterFunc) FilterFunc {
	return func(ctx context.Context, pod *v1.Pod) bool {
		return !filterFunc(ctx, pod)
	}
}

const podReadinessGracePeriod = 10 * time.Minute

// IsUnhealthy checks if a pod has not been ready since at least 10 minutes/
// This heuristic also detects infrastructure issues like not enough room to
// schedule pod. As false positives are less problematic than
// false negatives in our case, this is not a problem. if false positives were
// to be a frequent issue we could build a more specific heuristic by looking
// at the container statuses
func IsUnhealthy(_ context.Context, pod *v1.Pod) bool {
	// If the pod is terminating we ignore it and consider it healthy as it
	// should be gone soon.
	if pod.DeletionTimestamp != nil {
		return false
	}

	condition := getPodReadyCondition(&pod.Status)
	// if the pod has no ready condition, something is not ok
	// we consider it not healthy
	if condition == nil {
		return true
	}

	// if the pod is marked as ready it is healthy
	if condition.Status == v1.ConditionTrue {
		return false
	}

	// if the pod is marked unready but is still in the grace period
	// we don't consider it unhealthy yet
	return condition.LastTransitionTime.Add(podReadinessGracePeriod).Before(time.Now())
}

func getPodReadyCondition(status *v1.PodStatus) *v1.PodCondition {
	for _, condition := range status.Conditions {
		if condition.Type == v1.PodReady {
			return &condition
		}
	}
	return nil
}

// MustHaveControllerRevisionLabel checks if the pod is labeled with a controller
// revision hash and produces an error log if it is not.
func MustHaveControllerRevisionLabel(ctx context.Context, pod *v1.Pod) bool {
	log := ctrllog.FromContext(ctx).V(1)
	if (pod.Labels == nil) || (pod.Labels[appsv1.ControllerRevisionHashLabelKey] == "") {
		log.Error(
			trace.Errorf("pod does not have the '%s' label", appsv1.ControllerRevisionHashLabelKey),
			"ignoring malformed pod", "podName", pod.Name, "podLabels", pod.Labels,
		)
		return false
	}
	return true
}

// BelongsControllerRevisionFilter returns a FilterFunc checking if the pod belong
// to a specific controller revision.
func BelongsControllerRevisionFilter(controllerRevision string) FilterFunc {
	return func(_ context.Context, pod *v1.Pod) bool {
		return (pod.Labels != nil) && pod.Labels[appsv1.ControllerRevisionHashLabelKey] == controllerRevision
	}
}
