// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"sort"
	"testing"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
)

func Test_filterBuffer(t *testing.T) {
	log := logrus.New()
	data, err := os.ReadFile("testing/data/pod_list.json")
	require.NoError(t, err)

	_, decoder, err := newEncoderAndDecoderForContentType(responsewriters.DefaultContentType, newClientNegotiator())
	require.NoError(t, err)

	type args struct {
		allowedPods     []types.KubernetesResource
		contentEncoding string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Test filterBuffer with gzip",
			args: args{
				contentEncoding: "gzip",
				allowedPods: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: "default",
						Name:      "*",
					},
				},
			},
			want: []string{
				"default/nginx-deployment-6595874d85-6j2zm",
				"default/nginx-deployment-6595874d85-7xgmb",
				"default/nginx-deployment-6595874d85-c4kz8",
			},
		},
		{
			name: "Test filterBuffer with no gzip",
			args: args{
				contentEncoding: "",
				allowedPods: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: "default",
						Name:      "*",
					},
				},
			},
			want: []string{
				"default/nginx-deployment-6595874d85-6j2zm",
				"default/nginx-deployment-6595874d85-7xgmb",
				"default/nginx-deployment-6595874d85-c4kz8",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, decompress := newMemoryResponseWriter(t, data, tt.args.contentEncoding)

			err := filterBuffer(newPodFilterer(tt.args.allowedPods, nil, log), buf)
			require.NoError(t, err)

			// Decompress the buffer to compare the result.
			decompressedBuf := bytes.NewBuffer(nil)

			err = decompress(decompressedBuf, buf.Buffer())
			require.NoError(t, err)
			obj, _, err := decoder.Decode(decompressedBuf.Bytes(), nil, nil)
			require.NoError(t, err)
			switch o := obj.(type) {
			case *corev1.PodList:
				pods := collectPodsFromResponse(o)
				sort.Strings(pods)
				sort.Strings(tt.want)
				require.Equal(t, tt.want, pods)
			default:
				t.Errorf("filterBuffer() = %v, want %v", obj, &corev1.PodList{})
			}
		})
	}
}

func collectPodsFromResponse(podList *corev1.PodList) []string {
	pods := []string{}
	for _, pod := range podList.Items {
		pods = append(pods, pod.Namespace+"/"+pod.Name)
	}
	return pods
}

func newMemoryResponseWriter(t *testing.T, payload []byte, contentEncoding string) (*responsewriters.MemoryResponseWriter, decompressionFunc) {
	buf := responsewriters.NewMemoryResponseWriter()
	buf.Header().Set(contentEncodingHeader, contentEncoding)
	buf.Header().Set(responsewriters.ContentTypeHeader, responsewriters.DefaultContentType)

	switch contentEncoding {
	case "gzip":
		w, err := gzip.NewWriterLevel(buf, defaultGzipContentEncodingLevel)
		require.NoError(t, err)
		defer w.Close()
		defer w.Flush()
		_, err = w.Write(payload)
		require.NoError(t, err)
		return buf, func(dst io.Writer, src io.Reader) error {
			gzr, err := gzip.NewReader(src)
			if err != nil {
				return trace.Wrap(err)
			}
			defer gzr.Close()
			_, err = io.Copy(dst, gzr)
			return trace.Wrap(err)
		}
	default:
		buf.Write(payload)
		return buf, func(dst io.Writer, src io.Reader) error {
			_, err := io.Copy(dst, src)
			return trace.Wrap(err)
		}
	}
}
