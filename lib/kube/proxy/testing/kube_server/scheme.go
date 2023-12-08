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

package kubeserver

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
