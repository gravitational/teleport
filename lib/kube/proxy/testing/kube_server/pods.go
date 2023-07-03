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

package kubeserver

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
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

	namespace := p.ByName("podNamespace")
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
	namespace := p.ByName("podNamespace")
	name := p.ByName("podName")
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
	namespace := p.ByName("podNamespace")
	name := p.ByName("podName")
	deleteOpts, err := parseDeleteCollectionBody(req.Body)
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

func (s *KubeMockServer) DeletedPods(reqID string) []string {
	s.mu.Lock()
	key := deletedResource{kind: types.KindKubePod, requestID: reqID}
	deleted := make([]string, len(s.deletedResources[key]))
	copy(deleted, s.deletedResources[key])
	s.mu.Unlock()
	sort.Strings(deleted)
	return deleted
}

// parseDeleteCollectionBody parses the request body targeted to pod collection
// endpoints.
func parseDeleteCollectionBody(r io.Reader) (metav1.DeleteOptions, error) {
	into := metav1.DeleteOptions{}
	data, err := io.ReadAll(r)
	if err != nil {
		return into, trace.Wrap(err)
	}
	if len(data) == 0 {
		return into, nil
	}
	err = json.Unmarshal(data, &into)
	return into, trace.Wrap(err)
}
