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
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/podutils"
)

var (
	healthyStatus = v1.PodStatus{
		Conditions: []v1.PodCondition{
			{
				Type:   v1.PodReady,
				Status: v1.ConditionTrue,
			},
		},
	}
	unhealthyStatus = v1.PodStatus{
		Conditions: []v1.PodCondition{
			{
				Type:               v1.PodReady,
				Status:             v1.ConditionFalse,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour)),
			},
		},
	}
)

func podFixture(name, namespace string, labels map[string]string, status v1.PodStatus) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Status: status,
	}
}

func TestStatefulSetVersionUpdater_unblockStatefulSetRollout(t *testing.T) {

	ctx := context.Background()
	ns := validRandomResourceName("ns-")

	var (
		unmanagedHealthyPod   = podFixture("unmanaged-healthy", ns, map[string]string{"app": "unmanaged"}, healthyStatus)
		unmanagedUnhealthyPod = podFixture("unmanaged-unhealthy", ns, map[string]string{"app": "unmanaged"}, unhealthyStatus)
	)

	updateRevision := validRandomResourceName("sts-")
	currentRevision := validRandomResourceName("sts-")
	oldRevision := validRandomResourceName("sts-")

	defaultStatefulSet := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sts",
			Namespace: ns,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "managed"}},
		},
		Status: appsv1.StatefulSetStatus{
			CurrentRevision: currentRevision,
			UpdateRevision:  updateRevision,
		},
	}

	tests := []struct {
		name         string
		pods         v1.PodList
		sts          appsv1.StatefulSet
		expectedPods []string
		assertErr    require.ErrorAssertionFunc
	}{
		{
			name: "no statefulset managed pods",
			pods: v1.PodList{Items: []v1.Pod{unmanagedHealthyPod, unmanagedUnhealthyPod}},
			sts:  defaultStatefulSet,
			// No pod should be deleted
			expectedPods: []string{unmanagedHealthyPod.Name, unmanagedUnhealthyPod.Name},
			assertErr:    require.NoError,
		},
		{
			name: "statefulset managed pods all healthy",
			pods: v1.PodList{Items: []v1.Pod{
				podFixture("managed-healthy-1", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: currentRevision}, healthyStatus),
				podFixture("managed-healthy-2", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: updateRevision}, healthyStatus),
				podFixture("managed-healthy-3", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: oldRevision}, healthyStatus),
			}},
			sts:          defaultStatefulSet,
			expectedPods: []string{"managed-healthy-1", "managed-healthy-2", "managed-healthy-3"},
			assertErr:    require.NoError,
		},
		{
			name: "managed unhealthy pod from lastest revision",
			pods: v1.PodList{Items: []v1.Pod{
				podFixture("managed-healthy-1", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: currentRevision}, healthyStatus),
				podFixture("managed-unhealthy-1", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: updateRevision}, unhealthyStatus),
				podFixture("managed-unhealthy-2", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: updateRevision}, unhealthyStatus),
			}},
			sts: defaultStatefulSet,
			// No pod should be deleted as all unhealthy pods belong to the new revision
			expectedPods: []string{"managed-healthy-1", "managed-unhealthy-1", "managed-unhealthy-2"},
			assertErr:    require.NoError,
		},
		{
			name: "managed unhealthy pods from previous revision",
			pods: v1.PodList{Items: []v1.Pod{
				podFixture("managed-healthy-1", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: currentRevision}, healthyStatus),
				podFixture("managed-unhealthy-1", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: updateRevision}, unhealthyStatus),
				podFixture("managed-unhealthy-2", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: currentRevision}, unhealthyStatus),
				podFixture("managed-unhealthy-3", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: oldRevision}, unhealthyStatus),
			}},
			sts: defaultStatefulSet,
			// Managed unhealthy 2 and 3 should be deleted as they are unhealthy and belong to previous revisions
			// Managed unhealthy 1 should not be deleted as it belongs to the new revision
			expectedPods: []string{"managed-healthy-1", "managed-unhealthy-1"},
			assertErr:    require.NoError,
		},
		{
			name: "statefulset no revision",
			pods: v1.PodList{Items: []v1.Pod{
				podFixture("managed-healthy-1", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: currentRevision}, healthyStatus),
				podFixture("managed-unhealthy-1", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: updateRevision}, unhealthyStatus),
				podFixture("managed-unhealthy-2", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: currentRevision}, unhealthyStatus),
				podFixture("managed-unhealthy-3", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: oldRevision}, unhealthyStatus),
			}},
			sts: appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sts",
					Namespace: ns,
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "managed"}},
				},
				Status: appsv1.StatefulSetStatus{}, // Revisions not set
			},
			// If the revision is not set, it should not delete any pod
			expectedPods: []string{"managed-healthy-1", "managed-unhealthy-1", "managed-unhealthy-2", "managed-unhealthy-3"},
			assertErr:    require.Error,
		},
		{
			name: "managed no revision pod",
			pods: v1.PodList{Items: []v1.Pod{
				podFixture("managed-healthy-1", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: currentRevision}, healthyStatus),
				podFixture("managed-unhealthy-1", ns, map[string]string{"app": "managed", appsv1.ControllerRevisionHashLabelKey: currentRevision}, unhealthyStatus),
				podFixture("weird-unhealthy-1", ns, map[string]string{"app": "managed"}, unhealthyStatus),
			}},
			sts: defaultStatefulSet,
			// We should not touch the weird pods, only remove managed-unhealthy-1
			expectedPods: []string{"managed-healthy-1", "weird-unhealthy-1"},
			// We only log that something went wrong and continue
			assertErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Setup

			// Because the tests are deleting stuff and are not idempotent it's
			// easier to build the fake client per-test even if it is less
			// efficient than using a single client for all tests
			clientBuilder := fake.NewClientBuilder()
			clientBuilder.WithLists(&tt.pods)
			fakeClient := clientBuilder.Build()
			r := &StatefulSetVersionUpdater{
				VersionUpdater: VersionUpdater{},
				Client:         fakeClient,
				Scheme:         nil,
			}

			// Test execution
			err := r.unblockStatefulSetRolloutIfStuck(ctx, &tt.sts)
			tt.assertErr(t, err)

			// Analyzing the remaining pods to check if it deleted the right pods
			var remainingPodList v1.PodList
			err = fakeClient.List(ctx, &remainingPodList, client.InNamespace(ns))
			require.NoError(t, err)

			remainingPods := podutils.PodListToListOfPods(&remainingPodList)
			remainingPodNames := podutils.ListNames(remainingPods)
			slices.Sort(remainingPodNames)
			slices.Sort(tt.expectedPods)
			require.Equal(t, tt.expectedPods, remainingPodNames)
		})
	}
}
