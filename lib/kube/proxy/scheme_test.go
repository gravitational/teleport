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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
)

// TestNewClusterSchemaBuilder tests that newClusterSchemaBuilder doesn't panic
// when it's given types already registered in the global scheme.
func Test_newClusterSchemaBuilder(t *testing.T) {
	_, _, _, err := newClusterSchemaBuilder(logrus.StandardLogger(), &clientSet{})
	require.NoError(t, err)
}

type clientSet struct {
	kubernetes.Interface
	discovery.DiscoveryInterface
}

func (c *clientSet) Discovery() discovery.DiscoveryInterface {
	return c
}

var fakeAPIResource = metav1.APIResourceList{
	GroupVersion: "extensions/v1beta1",
	APIResources: []metav1.APIResource{
		{
			Name:       "ingresses",
			Kind:       "Ingress",
			Namespaced: true,
		},
	},
}

func (c *clientSet) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, []*metav1.APIResourceList{
		&fakeAPIResource,
	}, nil
}
