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
	"encoding/json"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"

	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
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

// Test_patchPodWithDebugContainer checks that the patch applied to a pod adds
// exactly one ephemeral container and that the added container is interactive
// (TTY). A strategic merge patch can append more than one entry to
// Spec.EphemeralContainers, so the validation must account for every container
// the patch adds rather than only the last one in the list.
func Test_patchPodWithDebugContainer(t *testing.T) {
	// Decoder built the same way as production code (ephemeral_containers.go),
	// using the package-global Kube codecs so no cluster or live API server is
	// required.
	codecs := globalKubeCodecs
	_, decoder, err := newEncoderAndDecoderForContentType(
		responsewriters.JSONContentType,
		newClientNegotiator(&codecs),
	)
	require.NoError(t, err)

	// Base pod with a single normal container and no ephemeral containers yet.
	basePod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "nginx"}},
		},
	}
	podJSON, err := json.Marshal(basePod)
	require.NoError(t, err)

	t.Run("single interactive container is accepted", func(t *testing.T) {
		patch := []byte(`{"spec":{"ephemeralContainers":[` +
			`{"name":"debugger","image":"alpine","tty":true}` +
			`]}}`)

		patchedPod, ephemeralContName, err := patchPodWithDebugContainer(
			decoder, podJSON, patch, basePod, apimachinerytypes.StrategicMergePatchType,
		)
		require.NoError(t, err)
		require.Len(t, patchedPod.Spec.EphemeralContainers, 1)
		require.Equal(t, "debugger", ephemeralContName)
	})

	t.Run("single non-interactive container is rejected", func(t *testing.T) {
		patch := []byte(`{"spec":{"ephemeralContainers":[` +
			`{"name":"debugger","image":"alpine","tty":false}` +
			`]}}`)

		_, _, err := patchPodWithDebugContainer(
			decoder, podJSON, patch, basePod, apimachinerytypes.StrategicMergePatchType,
		)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
	})

	t.Run("multiple added containers are rejected even when the last is interactive", func(t *testing.T) {
		// A single strategic merge patch adding two ephemeral containers: a
		// non-interactive one followed by an interactive one. Validating only
		// the last entry would accept this; the non-interactive container must
		// cause the whole patch to be rejected.
		patch := []byte(`{"spec":{"ephemeralContainers":[` +
			`{"name":"extra","image":"alpine","command":["/bin/sh","-c","id"],"tty":false},` +
			`{"name":"debugger","image":"alpine","tty":true}` +
			`]}}`)

		_, _, err := patchPodWithDebugContainer(
			decoder, podJSON, patch, basePod, apimachinerytypes.StrategicMergePatchType,
		)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
	})

	t.Run("multiple added containers are rejected when the last is non-interactive", func(t *testing.T) {
		patch := []byte(`{"spec":{"ephemeralContainers":[` +
			`{"name":"debugger","image":"alpine","tty":true},` +
			`{"name":"extra","image":"alpine","command":["/bin/sh","-c","id"],"tty":false}` +
			`]}}`)

		_, _, err := patchPodWithDebugContainer(
			decoder, podJSON, patch, basePod, apimachinerytypes.StrategicMergePatchType,
		)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
	})
}
