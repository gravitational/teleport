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
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/healthcheckconfig"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
)

func TestHealthCheckConfigService(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	tests := []struct {
		name string
		run  func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService)
	}{
		{
			name: "create",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				cfg1 := newHealthCfg(t, "cfg1")
				_, err := svc.CreateHealthCheckConfig(ctx, cfg1)
				require.NoError(t, err)

				cfg2 := newHealthCfg(t, "cfg2")
				_, err = svc.CreateHealthCheckConfig(ctx, cfg2)
				require.NoError(t, err)
			},
		},
		{
			name: "create invalid",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				cfg := newHealthCfg(t, "invalid")
				cfg.Spec = nil
				_, err := svc.CreateHealthCheckConfig(ctx, cfg)
				require.Error(t, err)
			},
		},
		{
			name: "create virtual defaults not persisted",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				for _, cfg := range newHealthCfgDefaults(t) {
					t.Run(cfg.GetMetadata().GetName(), func(t *testing.T) {
						// Create a virtual default which hasn't been written to the backend.
						out, err := svc.CreateHealthCheckConfig(ctx, cfg)
						require.NoError(t, err)
						require.Empty(t, cmp.Diff(cfg, out, protocmp.Transform()))
					})
				}
			},
		},
		{
			name: "create virtual default concurrent",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				errChan := make(chan error, 2)
				for i := 0; i < 2; i++ {
					go func() {
						cfg := services.VirtualDefaultHealthCheckConfigDB()
						_, err := svc.CreateHealthCheckConfig(ctx, cfg)
						errChan <- err
					}()
				}
				err1 := <-errChan
				err2 := <-errChan

				if err1 == nil {
					require.True(t, trace.IsAlreadyExists(err2))
				} else {
					require.NoError(t, err2)
					require.True(t, trace.IsAlreadyExists(err1))
				}
			},
		},
		{
			name: "get",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				cfg1 := newHealthCfg(t, "cfg1")
				_, err := svc.CreateHealthCheckConfig(ctx, cfg1)
				require.NoError(t, err)

				out, err := svc.GetHealthCheckConfig(ctx, cfg1.GetMetadata().GetName())
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(cfg1, out, protocmp.Transform()))
			},
		},
		{
			name: "get not found",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				_, err := svc.GetHealthCheckConfig(ctx, "not-found")
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name: "get virtual defaults not persisted",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				for name, cfg := range newHealthCfgDefaultsMap(t) {
					t.Run(name, func(t *testing.T) {
						// Get a virtual default which hasn't been written to the backend.
						out, err := svc.GetHealthCheckConfig(ctx, name)
						require.NoError(t, err)
						require.Empty(t, cmp.Diff(cfg, out, protocmp.Transform()))
					})
				}
			},
		},
		{
			name: "list with paging",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				cfg1 := newHealthCfg(t, "cfg1")
				cfg1, err := svc.CreateHealthCheckConfig(ctx, cfg1)
				require.NoError(t, err)
				cfg2 := newHealthCfg(t, "cfg2")
				cfg2, err = svc.CreateHealthCheckConfig(ctx, cfg2)
				require.NoError(t, err)

				var cfgs []*healthcheckconfigv1.HealthCheckConfig
				var token string
				for {
					out, nextToken, err := svc.ListHealthCheckConfigs(ctx, 1, token)
					require.NoError(t, err)
					require.Len(t, out, 1)
					cfgs = append(cfgs, out...)
					if nextToken == "" {
						break
					}
					token = nextToken
				}

				require.Len(t, cfgs, teleport.VirtualDefaultHealthCheckConfigCount+2)
				requireContainsHealthCheckConfigs(t, cfgs, cfg1, cfg2)
			},
		},
		{
			name: "list with paging splitting virtual defaults",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				// Create one regular config that sorts between virtual defaults
				// (assuming virtual defaults are named alphabetically)
				cfg := newHealthCfg(t, "aaa-config") // Sorts before virtual defaults
				_, err := svc.CreateHealthCheckConfig(ctx, cfg)
				require.NoError(t, err)

				pageSize := teleport.VirtualDefaultHealthCheckConfigCount
				page1, token, err := svc.ListHealthCheckConfigs(ctx, pageSize, "")
				require.NoError(t, err)
				require.Len(t, page1, pageSize)
				require.NotEmpty(t, token)

				page2, token2, err := svc.ListHealthCheckConfigs(ctx, pageSize, token)
				require.NoError(t, err)
				require.Len(t, page2, 1)
				require.Empty(t, token2)
				require.Empty(t, cmp.Diff(
					services.VirtualDefaultHealthCheckConfigKube(),
					page2[0], protocmp.Transform()))
			},
		},
		{
			name: "list virtual defaults not persisted",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				out, _, err := svc.ListHealthCheckConfigs(ctx, 10, "")
				require.NoError(t, err)
				require.Len(t, out, teleport.VirtualDefaultHealthCheckConfigCount)
				require.True(t, slices.IsSortedFunc(out, func(a, b *healthcheckconfigv1.HealthCheckConfig) int {
					return strings.Compare(a.GetMetadata().GetName(), b.GetMetadata().GetName())
				}), "expected virtual defaults to be sorted")
				requireEqualHealthCheckConfigs(t, out, newHealthCfgDefaults(t)...)
			},
		},
		{
			name: "update",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				cfg1 := newHealthCfg(t, "cfg1")
				cfg1, err := svc.CreateHealthCheckConfig(ctx, cfg1)
				require.NoError(t, err)

				cfg1.Spec.HealthyThreshold = 3
				cfgUpd, err := svc.UpdateHealthCheckConfig(ctx, cfg1)
				require.NoError(t, err)
				require.Equal(t, cfg1.Spec.HealthyThreshold, cfgUpd.Spec.HealthyThreshold)

				cfgGet, err := svc.GetHealthCheckConfig(ctx, cfg1.GetMetadata().GetName())
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(cfgUpd, cfgGet, protocmp.Transform()))

				cfgGet.Spec.HealthyThreshold = 9
				cfgUpd2, err := svc.UpdateHealthCheckConfig(ctx, cfgGet)
				require.NoError(t, err)
				require.Equal(t, cfgGet.Spec.HealthyThreshold, cfgUpd2.Spec.HealthyThreshold)
			},
		},
		{
			name: "update virtual defaults not persisted",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				for _, name := range healthCfgDefaultNames {
					t.Run(name, func(t *testing.T) {
						// Get a virtual default which hasn't been written to the backend,
						// then update.
						cfg, err := svc.GetHealthCheckConfig(ctx, name)
						require.NoError(t, err)
						cfg.Spec.Match.Disabled = true
						out, err := svc.UpdateHealthCheckConfig(ctx, cfg)
						require.NoError(t, err)
						require.Equal(t, cfg.Spec.Match.Disabled, out.Spec.Match.Disabled)
					})
				}
			},
		},
		{
			name: "update virtual defaults persisted",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				for _, cfg := range newHealthCfgDefaults(t) {
					t.Run(cfg.GetMetadata().GetName(), func(t *testing.T) {
						// Write a virtual default to the backend,
						// then update.
						cfgCrt, err := svc.CreateHealthCheckConfig(ctx, cfg)
						require.NoError(t, err)
						cfgCrt.Spec.Match.Disabled = true
						out, err := svc.UpdateHealthCheckConfig(ctx, cfgCrt)
						require.NoError(t, err)
						require.Empty(t, cmp.Diff(cfgCrt, out, protocmp.Transform()))
					})
				}
			},
		},
		{
			name: "upsert",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				cfg1 := newHealthCfg(t, "cfg1")
				cfgUps, err := svc.UpsertHealthCheckConfig(ctx, cfg1)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(cfgUps, cfg1, protocmp.Transform()))

				cfgGet, err := svc.GetHealthCheckConfig(ctx, cfgUps.GetMetadata().GetName())
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(cfgUps, cfgGet, protocmp.Transform()))

				cfgGet.Spec.HealthyThreshold = 9
				out, err := svc.UpsertHealthCheckConfig(ctx, cfgGet)
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(cfgGet, out, protocmp.Transform()))
			},
		},
		{
			name: "upsert virtual defaults",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				for _, cfg := range newHealthCfgDefaults(t) {
					t.Run(cfg.GetMetadata().GetName(), func(t *testing.T) {
						cfgUps, err := svc.UpsertHealthCheckConfig(ctx, cfg)
						require.NoError(t, err)
						require.Empty(t, cmp.Diff(cfgUps, cfg, protocmp.Transform()))

						cfgGet, err := svc.GetHealthCheckConfig(ctx, cfg.GetMetadata().GetName())
						require.NoError(t, err)
						require.Empty(t, cmp.Diff(cfgUps, cfgGet, protocmp.Transform()))

						cfgGet.Spec.Match.Disabled = true
						out, err := svc.UpsertHealthCheckConfig(ctx, cfgGet)
						require.NoError(t, err)
						require.Empty(t, cmp.Diff(cfgGet, out, protocmp.Transform()))
					})
				}
			},
		},
		{
			name: "delete",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				cfg1 := newHealthCfg(t, "cfg1")
				_, err := svc.CreateHealthCheckConfig(ctx, cfg1)
				require.NoError(t, err)

				err = svc.DeleteHealthCheckConfig(ctx, cfg1.GetMetadata().GetName())
				require.NoError(t, err)
			},
		},
		{
			name: "delete not found",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				err := svc.DeleteHealthCheckConfig(ctx, "not-found")
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name: "delete virtual defaults not persisted",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				for _, cfg := range newHealthCfgDefaults(t) {
					t.Run(cfg.GetMetadata().GetName(), func(t *testing.T) {
						// Delete a virtual default which hasn't been written to the backend,
						// then get the new virtual default which is unwritten to the backend.
						err := svc.DeleteHealthCheckConfig(ctx, cfg.GetMetadata().GetName())
						require.NoError(t, err)

						out, err := svc.GetHealthCheckConfig(ctx, cfg.GetMetadata().GetName())
						require.NoError(t, err)
						require.Empty(t, cmp.Diff(cfg, out, protocmp.Transform()))
					})
				}
			},
		},
		{
			name: "delete virtual defaults persisted",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				for _, cfg := range newHealthCfgDefaults(t) {
					t.Run(cfg.GetMetadata().GetName(), func(t *testing.T) {
						// Write a virtual default to the backend, then delete,
						// then get the new virtual default which is unwritten to the backend.
						_, err := svc.CreateHealthCheckConfig(ctx, cfg)
						require.NoError(t, err)

						err = svc.DeleteHealthCheckConfig(ctx, cfg.GetMetadata().GetName())
						require.NoError(t, err)

						out, err := svc.GetHealthCheckConfig(ctx, cfg.GetMetadata().GetName())
						require.NoError(t, err)
						requireEqualHealthCfg(t, cfg, out)
					})
				}
			},
		},
		{
			name: "delete all",
			run: func(t *testing.T, ctx context.Context, svc *HealthCheckConfigService) {
				_, err := svc.CreateHealthCheckConfig(ctx, newHealthCfg(t, "cfg1"))
				require.NoError(t, err)
				_, err = svc.CreateHealthCheckConfig(ctx, newHealthCfg(t, "cfg2"))
				require.NoError(t, err)
				_, err = svc.CreateHealthCheckConfig(ctx, newHealthCfg(t, "cfg3"))
				require.NoError(t, err)

				err = svc.DeleteAllHealthCheckConfigs(ctx)
				require.NoError(t, err)

				out, _, err := svc.ListHealthCheckConfigs(ctx, 10, "")
				require.NoError(t, err)
				require.Len(t, out, teleport.VirtualDefaultHealthCheckConfigCount)
				requireEqualHealthCheckConfigs(t, out, newHealthCfgDefaults(t)...)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t, ctx, newHealthSvc(t, ctx))
		})
	}
}

func newHealthSvc(t *testing.T, ctx context.Context) *HealthCheckConfigService {
	t.Helper()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		mem.Close()
	})
	svc, err := NewHealthCheckConfigService(backend.NewSanitizer(mem))
	require.NoError(t, err)
	return svc
}

func newHealthCfg(t *testing.T, name string) *healthcheckconfigv1.HealthCheckConfig {
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
		require.Equal(t, types.KindHealthCheckConfig, resource.GetKind())
	})

	t.Run("delete virtual default returns override error", func(t *testing.T) {
		for _, name := range healthCfgDefaultNames {
			t.Run(name, func(t *testing.T) {
				event := backend.Event{
					Type: types.OpDelete,
					Item: backend.Item{
						Key: backend.NewKey(healthCheckConfigPrefix, name),
					},
				}

				require.True(t, parser.match(event.Item.Key))

				_, err := parser.parse(event)

				var overrideErr parseEventOverrideError
				require.ErrorAs(t, err, &overrideErr,
					"deleting virtual default should return parseEventOverrideError")
				require.Len(t, overrideErr, 1)
				require.Equal(t, types.OpPut, overrideErr[0].Type)
				require.Equal(t, name, overrideErr[0].Resource.GetName())
				require.Equal(t, types.KindHealthCheckConfig, overrideErr[0].Resource.GetKind())
			})
		}
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
		require.Equal(t, types.KindHealthCheckConfig, resource.GetKind())
	})

}

func newHealthCfgDefaults(t *testing.T) []*healthcheckconfigv1.HealthCheckConfig {
	t.Helper()
	return []*healthcheckconfigv1.HealthCheckConfig{
		services.VirtualDefaultHealthCheckConfigDB(),
		services.VirtualDefaultHealthCheckConfigKube(),
	}
}

func newHealthCfgDefaultsMap(t *testing.T) map[string]*healthcheckconfigv1.HealthCheckConfig {
	t.Helper()
	return map[string]*healthcheckconfigv1.HealthCheckConfig{
		teleport.VirtualDefaultHealthCheckConfigDBName:   services.VirtualDefaultHealthCheckConfigDB(),
		teleport.VirtualDefaultHealthCheckConfigKubeName: services.VirtualDefaultHealthCheckConfigKube(),
	}
}

func requireEqualHealthCfg(t *testing.T, expect *healthcheckconfigv1.HealthCheckConfig, actual *healthcheckconfigv1.HealthCheckConfig) {
	t.Helper()
	require.Empty(t, cmp.Diff(expect, actual,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))
}

func requireEqualHealthCheckConfigs(t *testing.T, got []*healthcheckconfigv1.HealthCheckConfig, want ...*healthcheckconfigv1.HealthCheckConfig) {
	t.Helper()
	require.Empty(t,
		cmp.Diff(want, got,
			cmpopts.SortSlices(func(a, b *healthcheckconfigv1.HealthCheckConfig) bool {
				return a.GetMetadata().GetName() < b.GetMetadata().GetName()
			}),
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		),
	)
}

func requireContainsHealthCheckConfigs(t *testing.T, actual []*healthcheckconfigv1.HealthCheckConfig, expected ...*healthcheckconfigv1.HealthCheckConfig) {
	t.Helper()
	for _, exp := range expected {
		found := slices.ContainsFunc(actual, func(cfg *healthcheckconfigv1.HealthCheckConfig) bool {
			return cmp.Equal(exp, cfg, protocmp.Transform())
		})
		require.True(t, found, "config %q not found in list", exp.GetMetadata().GetName())
	}
}

var (
	healthCfgDefaultNames = []string{
		teleport.VirtualDefaultHealthCheckConfigDBName,
		teleport.VirtualDefaultHealthCheckConfigKubeName,
	}
)
