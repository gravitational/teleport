/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package common

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func runWorkloadIdentityCommand(
	t *testing.T, clt *authclient.Client, args []string,
) (*bytes.Buffer, error) {
	var stdoutBuf bytes.Buffer
	cmd := &WorkloadIdentityCommand{
		stdout: &stdoutBuf,
		now: func() time.Time {
			return time.Date(2024, 2, 5, 15, 4, 0, 0, time.UTC)
		},
	}
	return &stdoutBuf, runCommand(t, clt, cmd, args)
}

func TestWorkloadIdentity(t *testing.T) {
	t.Parallel()

	process := testenv.MakeTestServer(t, testenv.WithLogger(utils.NewSlogLoggerForTests()))
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	yamlData := `kind: workload_identity
version: v1
metadata:
  name: test
spec:
  spiffe:
    id: /test
`
	var expected workloadidentityv1pb.WorkloadIdentity
	require.NoError(t, yaml.Unmarshal([]byte(yamlData), &expected))

	t.Run("workload-identity ls empty", func(t *testing.T) {
		buf, err := runWorkloadIdentityCommand(
			t, rootClient, []string{
				"workload-identity", "ls",
			},
		)
		require.NoError(t, err)
		if golden.ShouldSet() {
			golden.Set(t, buf.Bytes())
		}
		require.Equal(t, string(golden.Get(t)), buf.String())
	})

	t.Run("resource list empty", func(t *testing.T) {
		buf, err := runResourceCommand(
			t, rootClient, []string{
				"get",
				types.KindWorkloadIdentity,
				"--format=json",
			},
		)
		require.NoError(t, err)

		resources := mustDecodeJSON[[]*workloadidentityv1pb.WorkloadIdentity](t, buf)
		require.Empty(t, resources)
	})

	t.Run("create", func(t *testing.T) {

		yamlPath := filepath.Join(t.TempDir(), "workload_identity.yaml")
		require.NoError(t, os.WriteFile(yamlPath, []byte(yamlData), 0644))
		_, err := runResourceCommand(t, rootClient, []string{"create", yamlPath})
		require.NoError(t, err)
	})

	t.Run("workload-identity ls", func(t *testing.T) {
		buf, err := runWorkloadIdentityCommand(
			t, rootClient, []string{
				"workload-identity", "ls",
			},
		)
		require.NoError(t, err)
		if golden.ShouldSet() {
			golden.Set(t, buf.Bytes())
		}
		require.Equal(t, string(golden.Get(t)), buf.String())
	})

	t.Run("resource list", func(t *testing.T) {
		buf, err := runResourceCommand(
			t, rootClient, []string{
				"get",
				types.KindWorkloadIdentity,
				"--format=json",
			},
		)
		require.NoError(t, err)

		resources := mustDecodeJSON[[]*workloadidentityv1pb.WorkloadIdentity](t, buf)
		require.NotEmpty(t, resources)
		require.Empty(t, cmp.Diff(
			[]*workloadidentityv1pb.WorkloadIdentity{&expected},
			resources,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
	})

	t.Run("workload-identity rm", func(t *testing.T) {
		buf, err := runWorkloadIdentityCommand(
			t, rootClient, []string{
				"workload-identity", "rm",
				expected.GetMetadata().GetName(),
			},
		)
		require.NoError(t, err)
		if golden.ShouldSet() {
			golden.Set(t, buf.Bytes())
		}
		require.Equal(t, string(golden.Get(t)), buf.String())
	})

	t.Run("resource list empty after delete", func(t *testing.T) {
		buf, err := runResourceCommand(
			t, rootClient, []string{
				"get",
				types.KindWorkloadIdentity,
				"--format=json",
			},
		)
		require.NoError(t, err)

		resources := mustDecodeJSON[[]*workloadidentityv1pb.WorkloadIdentity](t, buf)
		require.Empty(t, resources)
	})
}

func TestWorkloadIdentityRevocation(t *testing.T) {
	t.Parallel()

	process := testenv.MakeTestServer(t, testenv.WithLogger(utils.NewSlogLoggerForTests()))
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	t.Run("workload-identity revocations ls empty", func(t *testing.T) {
		buf, err := runWorkloadIdentityCommand(
			t, rootClient, []string{
				"workload-identity", "revocations", "ls",
			},
		)
		require.NoError(t, err)
		if golden.ShouldSet() {
			golden.Set(t, buf.Bytes())
		}
		require.Equal(t, string(golden.Get(t)), buf.String())
	})

	t.Run("get list empty", func(t *testing.T) {
		buf, err := runResourceCommand(
			t, rootClient, []string{
				"get",
				types.KindWorkloadIdentityX509Revocation,
				"--format=json",
			},
		)
		require.NoError(t, err)
		require.Equal(t, "[]", buf.String())
	})

	t.Run("workload-identity revocations add", func(t *testing.T) {
		buf, err := runWorkloadIdentityCommand(
			t, rootClient, []string{
				"workload-identity",
				"revocations",
				"add",
				"--type=x509",
				"--serial=aa:bb:cc:dd",
				"--reason=compromised",
				"--expires-at=2030-02-24T15:04:00Z",
			},
		)
		require.NoError(t, err)
		if golden.ShouldSet() {
			golden.Set(t, buf.Bytes())
		}
		require.Equal(t, string(golden.Get(t)), buf.String())
	})

	t.Run("workload-identity revocations ls with value", func(t *testing.T) {
		buf, err := runWorkloadIdentityCommand(
			t, rootClient, []string{
				"workload-identity", "revocations", "ls",
			},
		)
		require.NoError(t, err)
		if golden.ShouldSet() {
			golden.Set(t, buf.Bytes())
		}
		require.Equal(t, string(golden.Get(t)), buf.String())
	})

	t.Run("get list with value", func(t *testing.T) {
		buf, err := runResourceCommand(
			t, rootClient, []string{
				"get",
				types.KindWorkloadIdentityX509Revocation,
				"--format=json",
			},
		)
		require.NoError(t, err)

		resources := mustDecodeJSON[[]json.RawMessage](t, buf)
		require.Len(t, resources, 1)
		resource := &workloadidentityv1pb.WorkloadIdentityX509Revocation{}
		err = protojson.Unmarshal(resources[0], resource)
		require.NoError(t, err)

		require.Empty(t, cmp.Diff(
			&workloadidentityv1pb.WorkloadIdentityX509Revocation{
				Kind:    types.KindWorkloadIdentityX509Revocation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:    "aabbccdd",
					Expires: timestamppb.New(time.Date(2030, 2, 24, 15, 4, 0, 0, time.UTC)),
				},
				Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
					Reason:    "compromised",
					RevokedAt: timestamppb.New(time.Date(2024, 2, 5, 15, 4, 0, 0, time.UTC)),
				},
			},
			resource,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
	})

	t.Run("workload-identity revocations rm", func(t *testing.T) {
		buf, err := runWorkloadIdentityCommand(
			t, rootClient, []string{
				"workload-identity",
				"revocations",
				"rm",
				"--serial=aa:bb:cc:dd",
				"--type=x509",
			},
		)
		require.NoError(t, err)
		if golden.ShouldSet() {
			golden.Set(t, buf.Bytes())
		}
		require.Equal(t, string(golden.Get(t)), buf.String())
	})

	t.Run("get list empty after delete", func(t *testing.T) {
		buf, err := runResourceCommand(
			t, rootClient, []string{
				"get",
				types.KindWorkloadIdentityX509Revocation,
				"--format=json",
			},
		)
		require.NoError(t, err)
		require.Equal(t, "[]", buf.String())
	})
}
