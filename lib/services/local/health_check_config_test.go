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

package local

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/healthcheckconfig"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestHealthCheckConfigService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	service, err := NewHealthCheckConfigService(backend.NewSanitizer(mem))
	require.NoError(t, err)

	cfg1 := newHealthCheckConfig(t, "cfg1")
	cfg2 := newHealthCheckConfig(t, "cfg2")

	t.Run("empty", func(t *testing.T) {
		out, _, err := service.ListHealthCheckConfigs(ctx, 10, "")
		require.NoError(t, err)
		require.Empty(t, out)
	})

	t.Run("invalid resource is rejected", func(t *testing.T) {
		cfg := newHealthCheckConfig(t, "example")
		cfg.Spec = nil
		_, err := service.CreateHealthCheckConfig(ctx, cfg)
		require.Error(t, err)
	})

	t.Run("create", func(t *testing.T) {
		_, err := service.CreateHealthCheckConfig(ctx, cfg1)
		require.NoError(t, err)
		_, err = service.CreateHealthCheckConfig(ctx, cfg2)
		require.NoError(t, err)
	})

	t.Run("list", func(t *testing.T) {
		out, _, err := service.ListHealthCheckConfigs(ctx, 10, "")
		require.NoError(t, err)
		requireEqualHealthCheckConfigs(t, out, cfg1, cfg2)
	})

	t.Run("list with token", func(t *testing.T) {
		out1, token, err := service.ListHealthCheckConfigs(ctx, 1, "")
		require.NoError(t, err)
		require.NotEmpty(t, token)
		require.Len(t, out1, 1)
		out2, token, err := service.ListHealthCheckConfigs(ctx, 1, token)
		require.NoError(t, err)
		require.Empty(t, token)
		require.Len(t, out2, 1)

		combined := append(out1, out2...)
		require.Contains(t, combined, cfg1)
		require.Contains(t, combined, cfg2)
	})

	t.Run("get and update", func(t *testing.T) {
		out, err := service.GetHealthCheckConfig(ctx, cfg1.Metadata.Name)
		require.NoError(t, err)
		require.Equal(t, out, cfg1)

		out.Spec.HealthyThreshold = 3
		out, err = service.UpdateHealthCheckConfig(ctx, out)
		require.NoError(t, err)
		require.Equal(t, uint32(3), out.Spec.HealthyThreshold)
	})

	t.Run("delete not found", func(t *testing.T) {
		err := service.DeleteHealthCheckConfig(ctx, "asdf")
		require.IsType(t, trace.NotFound(""), err)
	})

	t.Run("delete", func(t *testing.T) {
		require.NoError(t, service.DeleteHealthCheckConfig(ctx, cfg1.Metadata.Name))
	})

	t.Run("upsert", func(t *testing.T) {
		cfg3 := newHealthCheckConfig(t, "cfg3")
		_, err := service.UpsertHealthCheckConfig(ctx, cfg3)
		require.NoError(t, err)

		out, _, err := service.ListHealthCheckConfigs(ctx, 10, "")
		require.NoError(t, err)
		requireEqualHealthCheckConfigs(t, out, cfg2, cfg3)
	})

	t.Run("delete all", func(t *testing.T) {
		require.NoError(t, service.DeleteAllHealthCheckConfigs(ctx))
		out, _, err := service.ListHealthCheckConfigs(ctx, 10, "")
		require.NoError(t, err)
		require.Empty(t, out)
	})
}

func newHealthCheckConfig(t *testing.T, name string) *healthcheckconfigv1.HealthCheckConfig {
	t.Helper()
	cfg, err := healthcheckconfig.NewHealthCheckConfig(name,
		&healthcheckconfigv1.HealthCheckConfigSpec{
			Match: &healthcheckconfigv1.Matcher{
				DbLabels: []*labelv1.Label{{
					Name:   types.Wildcard,
					Values: []string{types.Wildcard},
				}},
			},
		},
	)
	require.NoError(t, err)
	return cfg
}

func requireEqualHealthCheckConfigs(t *testing.T, got []*healthcheckconfigv1.HealthCheckConfig, want ...*healthcheckconfigv1.HealthCheckConfig) {
	t.Helper()
	cmpByName := func(a, b *healthcheckconfigv1.HealthCheckConfig) int {
		return strings.Compare(a.Metadata.GetName(), b.Metadata.GetName())
	}
	require.Empty(t, cmp.Diff(
		slices.SortedFunc(slices.Values(want), cmpByName),
		slices.SortedFunc(slices.Values(got), cmpByName),
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))
}

func TestHealthCheckConfigParser(t *testing.T) {
	t.Parallel()
	parser := newHealthCheckConfigParser()
	t.Run("delete", func(t *testing.T) {
		event := backend.Event{
			Type: types.OpDelete,
			Item: backend.Item{
				Key: backend.NewKey(healthCheckConfigPrefix, "example"),
			},
		}
		require.True(t, parser.match(event.Item.Key))
		resource, err := parser.parse(event)
		require.NoError(t, err)
		require.Equal(t, "example", resource.GetMetadata().Name)
	})
	t.Run("put", func(t *testing.T) {
		event := backend.Event{
			Type: types.OpPut,
			Item: backend.Item{
				Key: backend.NewKey(healthCheckConfigPrefix, "example"),
				Value: []byte(`
{
  "kind": "health_check_config",
  "version": "v1",
  "metadata": {
    "name": "example"
  },
  "spec": {
    "timeout": "30s",
    "interval": "60s",
    "healthy_threshold": 3,
    "unhealthy_threshold": 1,
    "match": {
      "db_labels": [
        {
          "name": "*",
          "values": [
            "*"
          ]
        }
      ]
    }
  }
}
`),
			},
		}
		require.True(t, parser.match(event.Item.Key))
		resource, err := parser.parse(event)
		require.NoError(t, err)
		require.Equal(t, "example", resource.GetMetadata().Name)
	})
}
