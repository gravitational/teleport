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

package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/protoadapt"

	"github.com/gravitational/teleport/api/utils"
)

type protoResource interface {
	Resource
	protoadapt.MessageV1
}

func TestCloning(t *testing.T) {
	// Test that cloning some of our messages produces the same type
	// with the same contents. When CheckAndSetDefaults sets an empty
	// slice or map instead of a nil one, set it to nil so the
	// equality check below won't fail.
	var resources []protoResource

	a, err := NewAccessRequest("foo", "bar", "role")
	require.NoError(t, err)
	accessRequest := a.(*AccessRequestV3)
	accessRequest.Spec.SuggestedReviewers = nil
	accessRequest.Spec.RequestedResourceIDs = nil
	resources = append(resources, accessRequest)

	user, err := NewUser("foo")
	require.NoError(t, err)
	resources = append(resources, user.(*UserV2))

	s, err := NewServer("foo", KindNode, ServerSpecV2{})
	require.NoError(t, err)
	server := s.(*ServerV2)
	server.Metadata.Labels = nil
	resources = append(resources, server)

	remCluster, err := NewRemoteCluster("foo")
	require.NoError(t, err)
	resources = append(resources, remCluster.(*RemoteClusterV3))

	for _, r := range resources {
		t.Run(fmt.Sprintf("%T", r), func(t *testing.T) {
			rCopy := utils.CloneProtoMsg(r)
			require.Equal(t, r, rCopy)
			require.IsType(t, r, rCopy)
		})
	}
}
