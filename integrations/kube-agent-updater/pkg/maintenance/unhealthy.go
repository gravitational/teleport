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

package maintenance

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	podReadinessGracePeriod = 10 * time.Minute
	deploymentKind          = "Deployment"
	statefulSetKind         = "StatefulSet"
)

// unhealthyWorkloadTrigger allows a maintenance to start if the workload is
// unhealthy. This is designed to recover faster if a new version breaks the
// agent. This way the user will not be left with a broken cluster until the
// next maintenance window.
type unhealthyWorkloadTrigger struct {
	name string
	kclient.Client
}

// Name returns the trigger name.
func (u unhealthyWorkloadTrigger) Name() string {
	return u.name
}

// CanStart implements maintenance.Trigger
func (u unhealthyWorkloadTrigger) CanStart(ctx context.Context, object kclient.Object) (bool, error) {
	switch workload := object.(type) {
	case *appsv1.Deployment:
		selector, err := metav1.LabelSelectorAsSelector(workload.Spec.Selector)
		if err != nil {
			return false, trace.Wrap(err)
		}
		return u.isWorkloadUnhealthy(ctx, workload.GetNamespace(), selector)
	case *appsv1.StatefulSet:
		selector, err := metav1.LabelSelectorAsSelector(workload.Spec.Selector)
		if err != nil {
			return false, trace.Wrap(err)
		}
		return u.isWorkloadUnhealthy(ctx, workload.GetNamespace(), selector)
	default:
		return false, trace.BadParameter(
			"workload type '%s' not supported",
			object.GetObjectKind().GroupVersionKind().String(),
		)
	}
}

// Default returns what to do if the trigger can't be evaluated.
func (u unhealthyWorkloadTrigger) Default() bool {
	return false
}

// isWorkloadUnhealthy checks the pods selected by a workload and returns true
// if at least one pod is unhealthy.
func (u unhealthyWorkloadTrigger) isWorkloadUnhealthy(ctx context.Context, namespace string, selector labels.Selector) (bool, error) {
	managedPods := &v1.PodList{}
	matchingSelector := kclient.MatchingLabelsSelector{Selector: selector}
	inNamespace := kclient.InNamespace(namespace)
	err := u.List(ctx, managedPods, inNamespace, matchingSelector)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// If the deployment manages no pods, it is considered unhealthy
	// and can be updated at any time
	if len(managedPods.Items) == 0 {
		return true, nil
	}

	// If at least a pod is unhealthy, we consider the whole workload unhealthy
	return len(UnhealthyPods(managedPods)) > 0, nil
}

// NewUnhealthyWorkloadTrigger triggers a maintenance if the watched workload
// is unhealthy.
func NewUnhealthyWorkloadTrigger(name string, client kclient.Client) Trigger {
	return unhealthyWorkloadTrigger{
		name:   name,
		Client: client,
	}
}

// UnhealthyPods takes a v1.PodList of pods and returns a list of all unhealthy
// pods.
func UnhealthyPods(list *v1.PodList) []*v1.Pod {
	var unhealthyPods []*v1.Pod
	for _, pod := range list.Items {
		if isPodUnhealthy(&pod) {
			unhealthyPods = append(unhealthyPods, &pod)
		}
	}
	return unhealthyPods
}

// A Pod is unhealthy if it is not Ready since at least X minutes
// This heuristic also detects infrastructure issues like not enough room to
// schedule pod. As false positives are less problematic than
// false negatives in our case, this is not a problem. If false positives were
// to be a frequent issue we could build a more specific heuristic by looking
// at the container statuses
func isPodUnhealthy(pod *v1.Pod) bool {
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
