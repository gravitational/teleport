// Copyright 2023 Gravitational, Inc
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

package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
)

type FakeResourceWithOrigin types.GithubConnector

type FakeKubernetesResource struct {
	client.Object
}

func (r FakeKubernetesResource) ToTeleport() FakeResourceWithOrigin {
	return nil
}

func (r FakeKubernetesResource) StatusConditions() *[]v1.Condition {
	return nil
}

type FakeKubernetesResourcePtrReceiver struct {
	client.Object
}

func (r *FakeKubernetesResourcePtrReceiver) ToTeleport() FakeResourceWithOrigin {
	return nil
}

func (r *FakeKubernetesResourcePtrReceiver) StatusConditions() *[]v1.Condition {
	return nil
}

func TestNewKubeResource(t *testing.T) {
	// Test with a value receiver
	resource := newKubeResource[FakeResourceWithOrigin, FakeKubernetesResource]()
	require.IsTypef(t, FakeKubernetesResource{}, resource, "Should be of type FakeKubernetesResource")
	require.NotNil(t, resource)

	// Test with a pointer receiver
	resourcePtr := newKubeResource[FakeResourceWithOrigin, *FakeKubernetesResourcePtrReceiver]()
	require.IsTypef(t, &FakeKubernetesResourcePtrReceiver{}, resourcePtr, "Should be a pointer on FakeKubernetesResourcePtrReceiver")
	require.NotNil(t, resourcePtr)
	require.NotNil(t, *resourcePtr)
}
