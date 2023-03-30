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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodListToListOfPods converts a v1.PodList coming from a pod LIST operation
// into a slice of pods.
func PodListToListOfPods(list *v1.PodList) (pods []*v1.Pod) {
	for _, pod := range list.Items {
		pod := pod
		pods = append(pods, &pod)
	}
	return
}

// ListNames takes any object list and returns a list of the object names.
// This can be used for logging or testing purposes.
func ListNames[T metav1.Object](objs []T) []string {
	names := make([]string, len(objs))
	for i, obj := range objs {
		names[i] = obj.GetName()
	}
	return names
}
