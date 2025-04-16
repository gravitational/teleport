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

package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport/lib/utils"
)

// TestNewClusterSchemaBuilder tests that newClusterSchemaBuilder doesn't panic
// when it's given types already registered in the global scheme.
func Test_newClusterSchemaBuilder(t *testing.T) {
	_, _, _, err := newClusterSchemaBuilder(utils.NewSlogLoggerForTests(), &clientSet{})
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
