/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/healthcheckconfig"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

func testCreateHealthCheckConfig(t *testing.T, clt *authclient.Client) {
	const resourceYAML = `kind: health_check_config
version: v1
metadata:
  name: testcfg
spec:
  match:
    db_labels:
    - name: '*'
      values:
      - '*'
    db_labels_expression: labels.env != "prod"
  interval: 60s
  timeout: 5s
  healthy_threshold: 2
  unhealthy_threshold: 1
`
	cfgRef := func(name string) string {
		return fmt.Sprintf("%v/%v", types.KindHealthCheckConfig, name)
	}
	// Get a specific non-existent resource
	_, err := runResourceCommand(t, clt, []string{"get", cfgRef("testcfg"), "--format=json"})
	require.ErrorContains(t, err, "doesn't exist")

	// Create the resource.
	resourceYAMLPath := filepath.Join(t.TempDir(), "resource.yaml")
	require.NoError(t, os.WriteFile(resourceYAMLPath, []byte(resourceYAML), 0644))
	_, err = runResourceCommand(t, clt, []string{"create", resourceYAMLPath})
	require.NoError(t, err)

	// Get the resource
	buf, err := runResourceCommand(t, clt, []string{"get", cfgRef("testcfg"), "--format=json"})
	require.NoError(t, err)

	rawResources := mustDecodeJSON[[]services.UnknownResource](t, buf)
	require.Len(t, rawResources, 1)
	resource, err := services.UnmarshalHealthCheckConfig(rawResources[0].Raw, services.DisallowUnknown())
	require.NoError(t, err)

	expectedJSON := mustTranscodeYAMLToJSON(t, bytes.NewReader([]byte(resourceYAML)))
	expected, err := services.UnmarshalHealthCheckConfig(expectedJSON, services.DisallowUnknown())
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(
		expected,
		resource,
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	))

	// Explicitly change the revision and try creating the resource with and
	// without the force flag.
	expected.GetMetadata().Revision = uuid.NewString()
	blob, err := services.MarshalHealthCheckConfig(expected, services.PreserveRevision())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(resourceYAMLPath, blob, 0644))

	_, err = runResourceCommand(t, clt, []string{"create", resourceYAMLPath})
	require.Error(t, err)
	require.IsType(t, trace.AlreadyExists(""), err)

	_, err = runResourceCommand(t, clt, []string{"create", "-f", resourceYAMLPath})
	require.NoError(t, err)
}

func testEditHealthCheckConfig(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()

	// create expected health check config
	expected, err := healthcheckconfig.NewHealthCheckConfig("test",
		&healthcheckconfigv1.HealthCheckConfigSpec{
			Match: &healthcheckconfigv1.Matcher{
				DbLabelsExpression: "labels.env == `dev`",
			},
		},
	)
	require.NoError(t, err)
	created, err := clt.CreateHealthCheckConfig(ctx, expected)
	require.NoError(t, err)

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}
		expected.GetMetadata().Revision = created.GetMetadata().GetRevision()
		collection := &healthCheckConfigCollection{
			items: []*healthcheckconfigv1.HealthCheckConfig{expected},
		}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	// Edit the AutoUpdateConfig resource.
	_, err = runEditCommand(t, clt, []string{"edit", "health_check_config/test"}, withEditor(editor))
	require.NoError(t, err)

	got, err := clt.GetHealthCheckConfig(ctx, "test")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, got,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))
}
