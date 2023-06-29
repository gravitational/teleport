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
	"fmt"
	"io"
	"path"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	authv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
)

func Test_filterBuffer(t *testing.T) {
	log := logrus.New()
	type objectAndAPI struct {
		obj string
		api string
	}
	teleToKubeResource := map[string]objectAndAPI{
		types.KindKubePod:                   {obj: "Pod", api: "v1"},
		types.KindKubeSecret:                {obj: "Secret", api: "v1"},
		types.KindKubeConfigmap:             {obj: "ConfigMap", api: "v1"},
		types.KindKubeService:               {obj: "Service", api: "v1"},
		types.KindKubeServiceAccount:        {obj: "ServiceAccount", api: "v1"},
		types.KindKubePersistentVolumeClaim: {obj: "PersistentVolumeClaim", api: "v1"},
		types.KindKubeDeployment:            {obj: "Deployment", api: "apps/v1"},
		types.KindKubeReplicaSet:            {obj: "ReplicaSet", api: "apps/v1"},
		types.KindKubeStatefulset:           {obj: "StatefulSet", api: "apps/v1"},
		types.KindKubeDaemonSet:             {obj: "DaemonSet", api: "apps/v1"},
		types.KindKubeRole:                  {obj: "Role", api: "rbac.authorization.k8s.io/v1"},
		types.KindKubeRoleBinding:           {obj: "RoleBinding", api: "rbac.authorization.k8s.io/v1"},
		types.KindKubeCronjob:               {obj: "CronJob", api: "batch/v1"},
		types.KindKubeJob:                   {obj: "Job", api: "batch/v1"},
		types.KindKubeIngress:               {obj: "Ingress", api: "networking.k8s.io/v1"},
	}

	_, decoder, err := newEncoderAndDecoderForContentType(responsewriters.DefaultContentType, newClientNegotiator())
	require.NoError(t, err)

	type args struct {
		contentEncoding string
		dataFile        string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "resource list compressed with gzip",
			args: args{
				dataFile:        "testing/data/resources_list.tmpl",
				contentEncoding: "gzip",
			},
			want: []string{
				"default/nginx-deployment-6595874d85-6j2zm",
				"default/nginx-deployment-6595874d85-7xgmb",
				"default/nginx-deployment-6595874d85-c4kz8",
			},
		},
		{
			name: "resource list uncompressed",
			args: args{
				dataFile:        "testing/data/resources_list.tmpl",
				contentEncoding: "",
			},
			want: []string{
				"default/nginx-deployment-6595874d85-6j2zm",
				"default/nginx-deployment-6595874d85-7xgmb",
				"default/nginx-deployment-6595874d85-c4kz8",
			},
		},
		{
			name: "table response compressed with gzip",
			args: args{
				dataFile:        "testing/data/partial_table.json",
				contentEncoding: "gzip",
			},
			want: []string{
				"default/kubernetes",
			},
		},
		{
			name: "table response uncompressed",
			args: args{
				dataFile:        "testing/data/partial_table.json",
				contentEncoding: "",
			},
			want: []string{
				"default/kubernetes",
			},
		},
	}
	for _, tt := range tests {
		for _, r := range types.KubernetesResourcesKinds {
			if slices.Contains(types.KubernetesClusterWideResourceKinds, r) {
				continue
			}
			r := r
			tt := tt
			allowedResources := []types.KubernetesResource{
				{
					Kind:      r,
					Namespace: "default",
					Name:      "*",
				},
			}
			t.Run(fmt.Sprintf("%s %s", r, tt.name), func(t *testing.T) {
				t.Parallel()
				temp, err := template.ParseFiles(tt.args.dataFile)
				require.NoError(t, err)
				data := &bytes.Buffer{}
				name := filepath.Base(tt.args.dataFile)
				err = temp.ExecuteTemplate(data, name, map[string]interface{}{
					"Kind": teleToKubeResource[r].obj,
					"API":  teleToKubeResource[r].api,
				},
				)
				require.NoError(t, err)

				buf, decompress := newMemoryResponseWriter(t, data.Bytes(), tt.args.contentEncoding)

				err = filterBuffer(newResourceFilterer(r, allowedResources, nil, log), buf)
				require.NoError(t, err)

				// Decompress the buffer to compare the result.
				decompressedBuf := bytes.NewBuffer(nil)

				err = decompress(decompressedBuf, buf.Buffer())
				require.NoError(t, err)
				obj, _, err := decoder.Decode(decompressedBuf.Bytes(), nil, nil)
				require.NoError(t, err)
				var resources []string
				switch o := obj.(type) {
				case *corev1.SecretList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *appsv1.DeploymentList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *appsv1.DaemonSetList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *appsv1.StatefulSetList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *authv1.RoleBindingList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *batchv1.CronJobList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *batchv1.JobList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *corev1.PodList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *corev1.ConfigMapList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *corev1.ServiceAccountList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *appsv1.ReplicaSetList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *corev1.ServiceList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *corev1.PersistentVolumeClaimList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *authv1.RoleList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *networkingv1.IngressList:
					resources = collectResourcesFromResponse(arrayToPointerArray(o.Items))
				case *metav1.Table:
					for i := range o.Rows {
						row := &(o.Rows[i])
						if row.Object.Object == nil {
							var err error
							// decode only if row.Object.Object was not decoded before.
							row.Object.Object, err = decodeAndSetGVK(decoder, row.Object.Raw)
							require.NoError(t, err)
						}

						resource, err := getKubeResourcePartialMetadataObject(r, row.Object.Object)
						require.NoError(t, err)
						resources = append(resources, resource.Namespace+"/"+resource.Name)
					}
				default:
					t.Errorf("filterBuffer() = %v (%T)", obj, obj)
				}
				require.ElementsMatch(t, tt.want, resources)
			})
		}
	}
}

func collectResourcesFromResponse[T kubeObjectInterface](originalList []T) []string {
	resources := make([]string, 0, len(originalList))
	for _, resource := range originalList {
		resources = append(resources, path.Join(resource.GetNamespace(), resource.GetName()))
	}
	return resources
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
