/*
Copyright 2022 Gravitational, Inc.

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

package proxy

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

var (
	// kubeScheme is the runtime Scheme that holds information about supported
	// message types.
	kubeScheme = runtime.NewScheme()
	// kubeCodecs creates a serializer/deserizalier for the different codecs
	// supported by the Kubernetes API.
	kubeCodecs = serializer.NewCodecFactory(kubeScheme)
)

// Register all groups in the schema's registry.
// It manually registers support for `metav1.Table` because go-client does not
// support it but `kubectl` calls require support for it.
func init() {
	// Register external types for Scheme
	metav1.AddToGroupVersion(kubeScheme, schema.GroupVersion{Version: "v1"})
	utilruntime.Must(metav1.AddMetaToScheme(kubeScheme))
	utilruntime.Must(metav1beta1.AddMetaToScheme(kubeScheme))
	utilruntime.Must(scheme.AddToScheme(kubeScheme))
	utilruntime.Must(kubeScheme.SetVersionPriority(corev1.SchemeGroupVersion))
}

// newClientNegotiator creates a negotiator that based on `Content-Type` header
// from the Kubernetes API response is able to create a different encoder/decoder.
// Supported content types:
// - "application/json"
// - "application/yaml"
// - "application/vnd.kubernetes.protobuf"
func newClientNegotiator() runtime.ClientNegotiator {
	return runtime.NewClientNegotiator(
		kubeCodecs.WithoutConversion(),
		schema.GroupVersion{
			// create a serializer for Kube API v1
			Version: "v1",
		},
	)
}
