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
