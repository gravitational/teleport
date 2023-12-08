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
