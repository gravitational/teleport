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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_isPodUnhealthy(t *testing.T) {
	now := metav1.Now()
	hourAgo := metav1.NewTime(time.Now().Add(-time.Hour))
	ctx := context.Background()

	tests := []struct {
		name string
		pod  *v1.Pod
		want bool
	}{
		{
			name: "ready",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
					},
				}},
			want: false,
		},
		{
			name: "unready but just deployed",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:               v1.PodReady,
							Status:             v1.ConditionFalse,
							LastTransitionTime: now,
						},
					},
				}},
			want: false,
		},
		{
			name: "unready but terminating",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now},
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:               v1.PodReady,
							Status:             v1.ConditionFalse,
							LastTransitionTime: hourAgo,
						},
					},
					StartTime: &hourAgo,
				}},
			want: false,
		},
		{
			// This can be imagePullBackOff, err image pull, crashloopBackOff, ...
			name: "stuck unready",
			pod: &v1.Pod{
				Status: v1.PodStatus{
					Conditions: []v1.PodCondition{
						{
							Type:               v1.PodReady,
							Status:             v1.ConditionFalse,
							LastTransitionTime: hourAgo,
						},
					},
				}},
			want: true,
		},
		{
			name: "no data",
			pod:  &v1.Pod{Status: v1.PodStatus{}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUnhealthy(ctx, tt.pod)
			require.Equal(t, tt.want, got)
		})
	}
}
