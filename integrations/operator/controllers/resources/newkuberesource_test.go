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
