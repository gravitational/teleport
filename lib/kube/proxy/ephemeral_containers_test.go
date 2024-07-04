/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
)

func Test_patchPod(t *testing.T) {
	type args struct {
		podData   []byte
		patchData []byte
		pod       corev1.Pod
		pt        apimachinerytypes.PatchType
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{
			name: "patch pod with json patch",
			args: args{
				podData:   []byte(`{"metadata":{"name":"test-pod"}}`),
				patchData: []byte(`[{"op":"replace","path":"/metadata/name","value":"new-test-pod"}]`),
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
					},
				},
				pt: apimachinerytypes.JSONPatchType,
			},
			want: []byte(`{"metadata":{"name":"new-test-pod"}}`),
		},
		{
			name: "patch pod with merge patch",
			args: args{
				podData:   []byte(`{"metadata":{"name":"test-pod"}}`),
				patchData: []byte(`{"metadata":{"name":"new-test-pod"}}`),
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
					},
				},
				pt: apimachinerytypes.MergePatchType,
			},
			want: []byte(`{"metadata":{"name":"new-test-pod"}}`),
		},
		{
			name: "patch pod with strategic patch json",
			args: args{
				podData:   []byte(`{"metadata":{"name":"test-pod"}}`),
				patchData: []byte(`{"metadata":{"name":"new-test-pod"}}`),
				pod: corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
					},
				},
				pt: apimachinerytypes.MergePatchType,
			},
			want: []byte(`{"metadata":{"name":"new-test-pod"}}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := patchPod(tt.args.podData, tt.args.patchData, tt.args.pod, tt.args.pt)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
