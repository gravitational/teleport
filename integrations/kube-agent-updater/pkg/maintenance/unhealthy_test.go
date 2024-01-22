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

package maintenance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var podReadyStatus = v1.PodStatus{
	Phase: v1.PodRunning,
	Conditions: []v1.PodCondition{
		{
			Type:   v1.PodReady,
			Status: v1.ConditionTrue,
		},
	}}
var podNotReadyStatus = v1.PodStatus{
	Phase: v1.PodRunning,
	Conditions: []v1.PodCondition{
		{
			Type:   v1.PodReady,
			Status: v1.ConditionFalse,
		},
	}}
var testPodSpec = v1.PodSpec{
	Containers: []v1.Container{{Name: "teleport", Image: "image"}},
}
var deploymentTypeMeta = metav1.TypeMeta{
	Kind:       "Deployment",
	APIVersion: "apps/v1",
}
var statefulsetTypeMeta = metav1.TypeMeta{
	Kind:       "StatefulSet",
	APIVersion: "apps/v1",
}

func TestUnhealthyWorkloadTrigger_CanStart(t *testing.T) {
	// The following section builds a fake client loaded with our fixtures.
	// It is not possible to use sigs.k8s.io/controller-runtime/pkg/envtest
	// because the Kubernetes api-server edits the pod status.
	namespace := "foo"

	fixtures := &v1.PodList{
		Items: []v1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "not-broken-replicated-1",
					Namespace: namespace,
					Labels: map[string]string{
						"broken": "no",
						"app":    "replicated",
					},
				},
				Spec:   testPodSpec,
				Status: podReadyStatus,
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "not-broken-replicated-2",
					Namespace: namespace,
					Labels: map[string]string{
						"broken": "no",
						"app":    "replicated",
					},
				},
				Spec:   testPodSpec,
				Status: podReadyStatus,
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "broken-replicated-1",
					Namespace: namespace,
					Labels: map[string]string{
						"broken": "yes",
						"app":    "replicated",
					},
				},
				Spec:   testPodSpec,
				Status: podNotReadyStatus,
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "broken-replicated-2",
					Namespace: namespace,
					Labels: map[string]string{
						"broken": "yes",
						"app":    "replicated",
					},
				},
				Spec:   testPodSpec,
				Status: podNotReadyStatus,
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "not-broken-single",
					Namespace: namespace,
					Labels: map[string]string{
						"broken": "no",
						"app":    "single",
					},
				},
				Spec:   testPodSpec,
				Status: podReadyStatus,
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "broken-single",
					Namespace: namespace,
					Labels: map[string]string{
						"broken": "yes",
						"app":    "single",
					},
				},
				Spec:   testPodSpec,
				Status: podNotReadyStatus,
			},
		},
	}
	clientBuilder := fake.NewClientBuilder()
	clientBuilder.WithLists(fixtures)
	client := clientBuilder.Build()
	ctx := context.Background()

	// Doing the real tests
	trigger := NewUnhealthyWorkloadTrigger("test-unhealthy", client)
	tests := []struct {
		name      string
		object    kclient.Object
		want      bool
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "deployment (replicated OK)",
			object: &appsv1.Deployment{
				TypeMeta:   deploymentTypeMeta,
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"broken": "no",
							"app":    "replicated",
						}}}},
			want:      false,
			assertErr: require.NoError,
		},
		{
			name: "statefulset (replicated OK)",
			object: &appsv1.StatefulSet{
				TypeMeta:   statefulsetTypeMeta,
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"broken": "no",
							"app":    "replicated",
						}}}},
			want:      false,
			assertErr: require.NoError,
		},
		{
			name: "replicated all KO",
			object: &appsv1.Deployment{
				TypeMeta:   deploymentTypeMeta,
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"broken": "yes",
							"app":    "replicated",
						}}}},
			want:      true,
			assertErr: require.NoError,
		},
		{
			name: "replicated mixed KO",
			object: &appsv1.Deployment{
				TypeMeta:   deploymentTypeMeta,
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "replicated",
						}}}},
			want:      true,
			assertErr: require.NoError,
		},
		{
			name: "single OK",
			object: &appsv1.Deployment{
				TypeMeta:   deploymentTypeMeta,
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"broken": "no",
							"app":    "single",
						}}}},
			want:      false,
			assertErr: require.NoError,
		},
		{
			name: "single KO",
			object: &appsv1.Deployment{
				TypeMeta:   deploymentTypeMeta,
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"broken": "yes",
							"app":    "single",
						}}}},
			want:      true,
			assertErr: require.NoError,
		},
		{
			name: "no pods",
			object: &appsv1.Deployment{
				TypeMeta:   deploymentTypeMeta,
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "no-match",
						}}}},
			want:      true,
			assertErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := trigger.CanStart(ctx, tt.object)
			tt.assertErr(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
