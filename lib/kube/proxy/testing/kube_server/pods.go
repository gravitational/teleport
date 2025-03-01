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
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
)

var podList = corev1.PodList{
	TypeMeta: metav1.TypeMeta{
		Kind:       "PodList",
		APIVersion: "v1",
	},
	ListMeta: metav1.ListMeta{
		ResourceVersion: "1231415",
	},
	Items: []corev1.Pod{
		newPod("nginx-1", "default"),
		newPod("nginx-2", "default"),
		newPod("test", "default"),
		newPod("nginx-1", "dev"),
		newPod("nginx-2", "dev"),
	},
}

func newPod(name, namespace string) corev1.Pod {
	return corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{},
	}
}

func (s *KubeMockServer) listPods(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	items := []corev1.Pod{}

	namespace := p.ByName("namespace")
	filter := func(pod corev1.Pod) bool {
		return len(namespace) == 0 || namespace == pod.Namespace
	}
	for _, pod := range podList.Items {
		if filter(pod) {
			items = append(items, pod)
		}
	}
	return &corev1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodList",
			APIVersion: "v1",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "1231415",
		},
		Items: items,
	}, nil
}

func (s *KubeMockServer) getPod(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	if s.getPodError != nil {
		s.writeResponseError(w, nil, s.getPodError.DeepCopy())
		return nil, nil
	}
	namespace := p.ByName("namespace")
	name := p.ByName("name")
	filter := func(pod corev1.Pod) bool {
		return pod.Name == name && namespace == pod.Namespace
	}
	for _, pod := range podList.Items {
		if filter(pod) {
			return pod, nil
		}
	}
	return nil, trace.NotFound("pod %q not found", filepath.Join(namespace, name))
}

func (s *KubeMockServer) deletePod(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	namespace := p.ByName("namespace")
	name := p.ByName("name")
	deleteOpts, err := parseDeleteCollectionBody(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqID := ""
	if deleteOpts.Preconditions != nil && deleteOpts.Preconditions.UID != nil {
		reqID = string(*deleteOpts.Preconditions.UID)
	}
	filter := func(pod corev1.Pod) bool {
		return pod.Name == name && namespace == pod.Namespace
	}
	for _, pod := range podList.Items {
		if filter(pod) {
			s.mu.Lock()
			s.deletedResources[deletedResource{kind: types.KindKubePod, requestID: reqID}] = append(s.deletedResources[deletedResource{kind: types.KindKubePod, requestID: reqID}], filepath.Join(namespace, name))
			s.mu.Unlock()
			return pod, nil
		}
	}
	return nil, trace.NotFound("pod %q not found", filepath.Join(namespace, name))
}

// parseDeleteCollectionBody parses the request body targeted to pod collection
// endpoints.
func parseDeleteCollectionBody(r *http.Request) (metav1.DeleteOptions, error) {
	into := metav1.DeleteOptions{}
	data, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return into, trace.Wrap(err)
	}
	if len(data) == 0 {
		return into, nil
	}
	decoder, err := newDecoderForContentType(r.Header.Get(responsewriters.ContentTypeHeader))
	if err != nil {
		return into, trace.Wrap(err)
	}
	objI, _, err := decoder.Decode(data, nil /* defaults */, &into)
	if err != nil {
		return into, trace.Wrap(err)
	}
	obj, ok := objI.(*metav1.DeleteOptions)
	if !ok {
		return into, trace.BadParameter("expected DeleteOptions, got %T", objI)
	}
	return *obj, trace.Wrap(err)
}

func newDecoderForContentType(contentType string) (runtime.Decoder, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, trace.Wrap(
			err,
			"unable to parse %q header %q",
			responsewriters.ContentTypeHeader,
			contentType,
		)
	}
	negotiator := newClientNegotiator()
	dec, err := negotiator.Decoder(mediaType, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return dec, nil
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

func (s *KubeMockServer) DeletedPods(reqID string) []string {
	s.mu.Lock()
	key := deletedResource{kind: types.KindKubePod, requestID: reqID}
	deleted := make([]string, len(s.deletedResources[key]))
	copy(deleted, s.deletedResources[key])
	s.mu.Unlock()
	sort.Strings(deleted)
	return deleted
}
