// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
package tbot

import (
	"context"
	"testing"

	"github.com/spiffe/aws-spiffe-workload-helper/vendoredaws"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func Test_renderAWSCreds(t *testing.T) {
	creds := &vendoredaws.CredentialProcessOutput{
		AccessKeyId:     "AKIAIOSFODNN7EXAMPLEAKID",
		SessionToken:    "AQoDYXdzEJrtyWJ4NjK7PiEXAMPLEST",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYzEXAMPLESAK",
	}
	ctx := context.Background()

	dest := &config.DestinationMemory{}
	require.NoError(t, dest.CheckAndSetDefaults())
	require.NoError(t, dest.Init(ctx, []string{}))

	err := renderAWSCreds(ctx, creds, dest)
	require.NoError(t, err)

	got, err := dest.Read(ctx, "aws_credentials")
	require.NoError(t, err)

	if golden.ShouldSet() {
		golden.Set(t, got)
	}
	require.Equal(t, golden.Get(t), got)
}
