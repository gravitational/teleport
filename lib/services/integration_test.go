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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestIntegrationMarshalCycle(t *testing.T) {
	ig, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "some-integration"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/DevTeams",
		},
	)
	require.NoError(t, err)

	bs, err := MarshalIntegration(ig)
	require.NoError(t, err)

	ig2, err := UnmarshalIntegration(bs)
	require.NoError(t, err)
	require.Equal(t, ig, ig2)
}

func TestIntegrationUnmarshal(t *testing.T) {
	ig, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "some-integration"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/DevTeams",
		},
	)
	require.NoError(t, err)

	storedBlob := []byte(`{"kind":"integration","sub_kind":"aws-oidc","version":"v1","metadata":{"name":"some-integration"},"spec":{"aws_oidc":{"role_arn":"arn:aws:iam::123456789012:role/DevTeams"}}}`)

	ig2, err := UnmarshalIntegration(storedBlob)
	require.NoError(t, err)
	require.NotNil(t, ig)

	require.Equal(t, ig, ig2)
}
