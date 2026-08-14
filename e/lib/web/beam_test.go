package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	beamservicev1 "github.com/gravitational/teleport/e/api/beamservice/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestListBeams(t *testing.T) {
	t.Parallel()

	t.Run("TestListBeamsWithNoParams", testListBeamsWithNoParams)
	t.Run("TestListBeamsWithUserFilter", testListBeamsWithUserFilter)
	t.Run("TestListBeamsWithPaging", testListBeamsWithPaging)
	t.Run("TestListBeamsWithSorting", testListBeamsWithSorting)
	t.Run("TestListBeamsNotEligable", testListBeamsNotEligable)
}

func testListBeamsWithNoParams(t *testing.T) {
	t.Parallel()

	for _, enableCache := range []bool{false, true} {
		t.Run(ternary(enableCache, "with cache", "without cache"), func(t *testing.T) {
			t.Parallel()

			env := newBeamTestEnv(t, true, withWebPackAuthCacheEnabled(enableCache))
			b := createBeam(t, env.suite, "1")

			resp, err := env.webPack.clt.Get(env.ctx, env.endpoint(), nil)
			require.NoError(t, err)

			var got listBeamsResponse
			err = json.Unmarshal(resp.Bytes(), &got)
			require.NoError(t, err)
			assert.Len(t, got.Items, 1)
			assert.Empty(t, cmp.Diff(got.Items, []beamDetails{
				{
					Name:           b.GetMetadata().GetName(),
					Alias:          b.GetStatus().GetAlias(),
					User:           b.GetStatus().GetUser(),
					Expires:        b.GetSpec().GetExpires().AsTime(),
					NodeId:         b.GetStatus().GetNodeId(),
					AppName:        b.GetStatus().GetAppName(),
					EgressMode:     "unrestricted",
					AllowedDomains: []string{},
				},
			}))
		})
	}
}

func testListBeamsWithUserFilter(t *testing.T) {
	t.Parallel()

	for _, enableCache := range []bool{false, true} {
		t.Run(ternary(enableCache, "with cache", "without cache"), func(t *testing.T) {
			t.Parallel()

			env := newBeamTestEnv(t, true, withWebPackAuthCacheEnabled(enableCache))
			createBeam(t, env.suite, "1")
			b2 := createBeam(t, env.suite, "2")

			resp, err := env.webPack.clt.Get(env.ctx, env.endpoint(), url.Values{
				"user": []string{"user-2@example.com"},
			})
			require.NoError(t, err)

			var got listBeamsResponse
			err = json.Unmarshal(resp.Bytes(), &got)
			require.NoError(t, err)
			assert.Len(t, got.Items, 1)
			assert.Empty(t, cmp.Diff(got.Items, []beamDetails{
				{
					Name:           b2.GetMetadata().GetName(),
					Alias:          b2.GetStatus().GetAlias(),
					User:           b2.GetStatus().GetUser(),
					Expires:        b2.GetSpec().GetExpires().AsTime(),
					NodeId:         b2.GetStatus().GetNodeId(),
					AppName:        b2.GetStatus().GetAppName(),
					EgressMode:     "unrestricted",
					AllowedDomains: []string{},
				},
			}))
		})
	}
}

func testListBeamsWithPaging(t *testing.T) {
	t.Parallel()

	for _, enableCache := range []bool{false, true} {
		t.Run(ternary(enableCache, "with cache", "without cache"), func(t *testing.T) {
			t.Parallel()

			env := newBeamTestEnv(t, true, withWebPackAuthCacheEnabled(enableCache))
			b1 := createBeam(t, env.suite, "1")
			b2 := createBeam(t, env.suite, "2")
			b3 := createBeam(t, env.suite, "3")

			sortField := ternary(enableCache, "expires", "name")
			resp, err := env.webPack.clt.Get(env.ctx, env.endpoint(), url.Values{
				"page_size":  []string{"2"},
				"sort_field": []string{sortField},
				"sort_dir":   []string{"asc"},
			})
			require.NoError(t, err)

			var firstPage listBeamsResponse
			err = json.Unmarshal(resp.Bytes(), &firstPage)
			require.NoError(t, err)
			require.Len(t, firstPage.Items, 2)
			require.NotEmpty(t, firstPage.NextPageToken)
			assert.Equal(t, []beamDetails{
				{
					Name:           b1.GetMetadata().GetName(),
					Alias:          b1.GetStatus().GetAlias(),
					User:           b1.GetStatus().GetUser(),
					Expires:        b1.GetSpec().GetExpires().AsTime(),
					NodeId:         b1.GetStatus().GetNodeId(),
					AppName:        b1.GetStatus().GetAppName(),
					EgressMode:     "unrestricted",
					AllowedDomains: []string{},
				},
				{
					Name:           b2.GetMetadata().GetName(),
					Alias:          b2.GetStatus().GetAlias(),
					User:           b2.GetStatus().GetUser(),
					Expires:        b2.GetSpec().GetExpires().AsTime(),
					NodeId:         b2.GetStatus().GetNodeId(),
					AppName:        b2.GetStatus().GetAppName(),
					EgressMode:     "unrestricted",
					AllowedDomains: []string{},
				},
			}, firstPage.Items)

			resp, err = env.webPack.clt.Get(env.ctx, env.endpoint(), url.Values{
				"page_size":  []string{"2"},
				"page_token": []string{firstPage.NextPageToken},
				"sort_field": []string{sortField},
				"sort_dir":   []string{"asc"},
			})
			require.NoError(t, err)

			var secondPage listBeamsResponse
			err = json.Unmarshal(resp.Bytes(), &secondPage)
			require.NoError(t, err)
			assert.Equal(t, []beamDetails{
				{
					Name:           b3.GetMetadata().GetName(),
					Alias:          b3.GetStatus().GetAlias(),
					User:           b3.GetStatus().GetUser(),
					Expires:        b3.GetSpec().GetExpires().AsTime(),
					NodeId:         b3.GetStatus().GetNodeId(),
					AppName:        b3.GetStatus().GetAppName(),
					EgressMode:     "unrestricted",
					AllowedDomains: []string{},
				},
			}, secondPage.Items)
			assert.Empty(t, secondPage.NextPageToken)
		})
	}
}

func testListBeamsWithSorting(t *testing.T) {
	t.Parallel()

	env := newBeamTestEnv(t, true, withWebPackAuthCacheEnabled(true))

	expires := time.Now().Add(24 * time.Hour)

	b1 := createBeam(t, env.suite, "1", func(beam *beamsv1.Beam) {
		beam.GetMetadata().SetName("222")
		beam.GetStatus().SetAlias("beam-alpha")
		beam.GetStatus().SetUser("user-3@example.com")
		beam.GetSpec().SetExpires(timestamppb.New(expires.Add(3 * time.Millisecond)))
	})
	b2 := createBeam(t, env.suite, "2", func(beam *beamsv1.Beam) {
		beam.GetMetadata().SetName("333")
		beam.GetStatus().SetAlias("beam-charlie")
		beam.GetStatus().SetUser("user-2@example.com")
		beam.GetSpec().SetExpires(timestamppb.New(expires.Add(1 * time.Millisecond)))
	})
	b3 := createBeam(t, env.suite, "3", func(beam *beamsv1.Beam) {
		beam.GetMetadata().SetName("111")
		beam.GetStatus().SetAlias("beam-beta")
		beam.GetStatus().SetUser("user-1@example.com")
		beam.GetSpec().SetExpires(timestamppb.New(expires.Add(2 * time.Millisecond)))
	})

	testCases := []struct {
		name        string
		field       string
		dir         string
		ordered     []any
		expectError string
	}{
		{
			name:    "by-name-asc",
			field:   "name",
			dir:     "AsC",
			ordered: []any{"111", "222", "333"},
		},
		{
			name:    "by-name-desc",
			field:   "name",
			dir:     "dEsC",
			ordered: []any{"333", "222", "111"},
		},
		{
			name:    "by-alias-asc",
			field:   "alias",
			dir:     "asc",
			ordered: []any{"beam-alpha", "beam-beta", "beam-charlie"},
		},
		{
			name:    "by-alias-desc",
			field:   "alias",
			dir:     "desc",
			ordered: []any{"beam-charlie", "beam-beta", "beam-alpha"},
		},
		{
			name:    "by-user-asc",
			field:   "user",
			dir:     "asc",
			ordered: []any{"user-1@example.com", "user-2@example.com", "user-3@example.com"},
		},
		{
			name:    "by-user-desc",
			field:   "user",
			dir:     "desc",
			ordered: []any{"user-3@example.com", "user-2@example.com", "user-1@example.com"},
		},
		{
			name:  "by-expires-asc",
			field: "expires",
			dir:   "asc",
			ordered: []any{
				b2.GetSpec().GetExpires().AsTime(),
				b3.GetSpec().GetExpires().AsTime(),
				b1.GetSpec().GetExpires().AsTime(),
			},
		},
		{
			name:  "by-expires-desc",
			field: "expires",
			dir:   "desc",
			ordered: []any{
				b1.GetSpec().GetExpires().AsTime(),
				b3.GetSpec().GetExpires().AsTime(),
				b2.GetSpec().GetExpires().AsTime(),
			},
		},
		{
			name:        "invalid-field",
			field:       "invalid",
			expectError: `unsupported sort field "invalid" but expected name, alias, user or expires`,
		},
		{
			name:        "invalid-order",
			dir:         "invalid",
			expectError: `unsupported sort order "invalid" but expected asc or desc`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp, err := env.webPack.clt.Get(env.ctx, env.endpoint(), url.Values{
				"sort_field": []string{tc.field},
				"sort_dir":   []string{tc.dir},
			})
			if tc.expectError != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.expectError)
				return
			}
			require.NoError(t, err)

			var got listBeamsResponse
			err = json.Unmarshal(resp.Bytes(), &got)
			require.NoError(t, err)

			assert.Len(t, got.Items, 3)

			for i, beam := range got.Items {
				expected := tc.ordered[i]
				switch tc.field {
				case "name":
					assert.Equal(t, expected, beam.Name)
				case "alias":
					assert.Equal(t, expected, beam.Alias)
				case "user":
					assert.Equal(t, expected, beam.User)
				case "expires":
					assert.Equal(t, expected, beam.Expires)
				}
			}
		})
	}
}

func testListBeamsNotEligable(t *testing.T) {
	t.Parallel()

	env := newBeamTestEnv(t, false)
	_, err := env.webPack.clt.Get(env.ctx, env.endpoint(), nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "not authorized to use beams")
}

func createAdminUser(t *testing.T, s *webSuite) {
	t.Helper()

	ctx := t.Context()

	role, err := types.NewRole(
		"beam-admin",
		types.RoleSpecV6{
			Allow: types.RoleConditions{
				BeamLabels: types.Labels{
					types.BeamOwnerLabel: {types.Wildcard},
				},
				Rules: []types.Rule{
					{
						Resources: []string{types.KindBeam},
						Verbs:     []string{types.Wildcard},
					},
				},
			},
		},
	)
	require.NoError(t, err)

	role, err = s.testAuthServer.Auth().CreateRole(ctx, role)
	require.NoError(t, err)

	createUserWithOpts(t, s, "admin", withRoles(role.GetName()), withPassword())
}

func createBeam(t *testing.T, s *webSuite, id string, options ...func(beam *beamsv1.Beam)) *beamsv1.Beam {
	t.Helper()

	ctx := t.Context()

	name := fmt.Sprintf("beam-%s-id", id)
	expiresMeta := time.Now().Add(24 * time.Hour).Add(5 * time.Minute)
	expiresSpec := time.Now().Add(24 * time.Hour)
	user := fmt.Sprintf("user-%s@example.com", id)
	alias := aliasForId(id)

	pre := beamsv1.Beam_builder{
		Kind:    "beam",
		Version: "v1",
		Metadata: headerv1.Metadata_builder{
			Name:    name,
			Expires: timestamppb.New(expiresMeta),
			Labels: map[string]string{
				types.BeamIDLabel:    name,
				types.BeamOwnerLabel: user,
				types.BeamAliasLabel: alias,
			},
		}.Build(),
		Spec: beamsv1.BeamSpec_builder{
			AllowedDomains: []string{},
			Expires:        timestamppb.New(expiresSpec),
			Egress:         beamsv1.EgressMode_EGRESS_MODE_UNRESTRICTED,
		}.Build(),
		Status: beamsv1.BeamStatus_builder{
			User:    user,
			Alias:   alias,
			NodeId:  fmt.Sprintf("beam-%s-node", id),
			AppName: fmt.Sprintf("beam-%s-app", id),
		}.Build(),
	}.Build()

	for _, opt := range options {
		opt(pre)
	}

	actions, err := s.testAuthServer.AuthServer.AuthServer.AppendPutBeamActions(nil, pre, backend.NotExists())
	require.NoError(t, err)

	_, err = s.testAuthServer.AuthServer.Backend.AtomicWrite(ctx, actions)
	require.NoError(t, err)

	// Wait for changes to propagate to the cache
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		_, err := s.testAuthServer.AuthServer.AuthServer.GetBeam(ctx, pre.GetMetadata().GetName())
		require.NoError(collect, err)
	}, 10*time.Second, 100*time.Millisecond)

	beam, err := s.testAuthServer.AuthServer.AuthServer.GetBeam(ctx, pre.GetMetadata().GetName())
	require.NoError(t, err)
	return beam
}

func aliasForId(id string) string {
	switch id {
	case "1":
		return "beam-alpha"
	case "2":
		return "beam-beta"
	case "3":
		return "beam-charlie"
	default:
		return "beam-alias"
	}
}

func ternary[T any](b bool, t, f T) T {
	if b {
		return t
	}
	return f
}

func TestCreateBeam(t *testing.T) {
	t.Parallel()

	t.Run("entitled creates beam", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, true)

		resp, err := env.webPack.clt.PostJSON(env.ctx, env.endpoint(), map[string]any{})
		require.NoError(t, err)

		var got beamDetails
		require.NoError(t, json.Unmarshal(resp.Bytes(), &got))
		assert.NotEmpty(t, got.Name)
		assert.Equal(t, "unrestricted", got.EgressMode)
		assert.Equal(t, "provision_complete", got.ComputeStatus)
		assert.Nil(t, got.Publish)

		provisionReqs := env.compute.getProvisionRequests()
		require.Len(t, provisionReqs, 1)
		assert.Equal(t, got.Name, provisionReqs[0].GetBeamId())
	})

	t.Run("entitled creates restricted beam", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, true)

		resp, err := env.webPack.clt.PostJSON(env.ctx, env.endpoint(), map[string]any{
			"egress_mode":     "restricted",
			"allowed_domains": []string{"example.com."},
		})
		require.NoError(t, err)

		var got beamDetails
		require.NoError(t, json.Unmarshal(resp.Bytes(), &got))
		assert.Equal(t, "restricted", got.EgressMode)
		assert.Equal(t, []string{"example.com."}, got.AllowedDomains)
	})

	t.Run("invalid egress mode is rejected", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, true)
		_, err := env.webPack.clt.PostJSON(env.ctx, env.endpoint(), map[string]any{
			"egress_mode": "bogus",
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "unsupported egress mode")
	})

	t.Run("not entitled", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, false)
		_, err := env.webPack.clt.PostJSON(env.ctx, env.endpoint(), map[string]any{})
		require.Error(t, err)
		require.ErrorContains(t, err, "not authorized to use beams")
	})
}

func TestGetBeam(t *testing.T) {
	t.Parallel()

	t.Run("entitled", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, true)

		unpublished := createBeam(t, env.suite, "1")
		published := createBeam(t, env.suite, "2", func(b *beamsv1.Beam) {
			b.GetSpec().SetPublish(beamsv1.PublishSpec_builder{
				Port:     beamPublishPort,
				Protocol: beamsv1.Protocol_PROTOCOL_HTTP,
			}.Build())
		})

		tests := []struct {
			name        string
			beamName    string
			wantErr     string
			wantPublish *beamPublishConfig
		}{
			{
				name:     "unpublished",
				beamName: unpublished.GetMetadata().GetName(),
			},
			{
				name:        "published HTTP",
				beamName:    published.GetMetadata().GetName(),
				wantPublish: &beamPublishConfig{Port: beamPublishPort, Protocol: "http"},
			},
			{
				name:     "not found",
				beamName: "does-not-exist",
				wantErr:  `beam "does-not-exist" doesn't exist`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp, err := env.webPack.clt.Get(env.ctx, env.endpoint(tt.beamName), nil)

				if tt.wantErr != "" {
					require.Error(t, err)
					require.ErrorContains(t, err, tt.wantErr)
					return
				}
				require.NoError(t, err)

				var got beamDetails
				require.NoError(t, json.Unmarshal(resp.Bytes(), &got))
				assert.Equal(t, tt.beamName, got.Name)
				assert.Equal(t, "unrestricted", got.EgressMode)
				assert.Empty(t, got.AllowedDomains)
				assert.Equal(t, tt.wantPublish, got.Publish)
				if tt.wantPublish == nil {
					assert.NotContains(t, string(resp.Bytes()), `"publish"`)
				}
			})
		}
	})

	t.Run("not entitled", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, false)
		_, err := env.webPack.clt.Get(env.ctx, env.endpoint("any-name"), nil)
		require.Error(t, err)
		require.ErrorContains(t, err, "not authorized to use beams")
	})
}

func TestUpdateBeam(t *testing.T) {
	t.Parallel()

	t.Run("entitled", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, true)

		tests := []struct {
			name         string
			id           string
			alias        string
			initialState func(*beamsv1.Beam)
			body         beamDetails
			wantPublish  *beamPublishConfig
			wantErr      string
		}{
			{
				name:  "omitting publish unpublishes the beam",
				id:    "u1",
				alias: "unpublish-beam",
				initialState: func(b *beamsv1.Beam) {
					b.GetSpec().SetPublish(beamsv1.PublishSpec_builder{
						Port:     beamPublishPort,
						Protocol: beamsv1.Protocol_PROTOCOL_HTTP,
					}.Build())
				},
				body:        beamDetails{},
				wantPublish: nil,
			},
			{
				name:        "publish as HTTP app on port 8080",
				id:          "u2",
				alias:       "publish-http",
				body:        beamDetails{Publish: &beamPublishConfig{Port: 8080, Protocol: "http"}},
				wantPublish: &beamPublishConfig{Port: beamPublishPort, Protocol: "http"},
			},
			{
				name:        "publish as TCP app on port 8080",
				id:          "u2-tcp",
				alias:       "publish-tcp",
				body:        beamDetails{Publish: &beamPublishConfig{Port: 8080, Protocol: "tcp"}},
				wantPublish: &beamPublishConfig{Port: beamPublishPort, Protocol: "tcp"},
			},
			{
				name:        "publish defaults protocol and port when omitted",
				id:          "u2-default",
				alias:       "publish-default",
				body:        beamDetails{Publish: &beamPublishConfig{}},
				wantPublish: &beamPublishConfig{Port: beamPublishPort, Protocol: "http"},
			},
			{
				name:    "publish with unsupported protocol is rejected",
				id:      "u4",
				alias:   "publish-bad",
				body:    beamDetails{Publish: &beamPublishConfig{Protocol: "ftp"}},
				wantErr: "unsupported protocol",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				beam := createBeam(t, env.suite, tt.id, func(b *beamsv1.Beam) {
					b.GetMetadata().ClearExpires()
					b.GetStatus().SetAlias(tt.alias)
					b.GetMetadata().GetLabels()[types.BeamAliasLabel] = tt.alias
					if tt.initialState != nil {
						tt.initialState(b)
					}
				})

				resp, err := env.webPack.clt.PutJSON(env.ctx, env.endpoint(beam.GetMetadata().GetName()), tt.body)
				if tt.wantErr != "" {
					require.Error(t, err)
					require.ErrorContains(t, err, tt.wantErr)
					return
				}
				require.NoError(t, err)

				var got beamDetails
				require.NoError(t, json.Unmarshal(resp.Bytes(), &got))
				assert.Equal(t, beam.GetMetadata().GetName(), got.Name)
				assert.Equal(t, tt.wantPublish, got.Publish)
			})
		}
	})

	t.Run("updates allowed domains for a restricted beam", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, true)
		beam := createBeam(t, env.suite, "1", func(b *beamsv1.Beam) {
			b.GetMetadata().ClearExpires()
			b.GetSpec().SetEgress(beamsv1.EgressMode_EGRESS_MODE_RESTRICTED)
			b.GetSpec().SetAllowedDomains([]string{"old.example.com."})
		})

		resp, err := env.webPack.clt.PutJSON(env.ctx, env.endpoint(beam.GetMetadata().GetName()), beamDetails{
			EgressMode:     "restricted",
			AllowedDomains: []string{"new.example.com."},
		})
		require.NoError(t, err)

		var got beamDetails
		require.NoError(t, json.Unmarshal(resp.Bytes(), &got))
		assert.Equal(t, "restricted", got.EgressMode)
		assert.Equal(t, []string{"new.example.com."}, got.AllowedDomains)
	})

	t.Run("changing egress mode is rejected by the service", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, true)
		beam := createBeam(t, env.suite, "1", func(b *beamsv1.Beam) {
			b.GetMetadata().ClearExpires()
		})

		_, err := env.webPack.clt.PutJSON(env.ctx, env.endpoint(beam.GetMetadata().GetName()), beamDetails{
			EgressMode: "restricted",
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "egress")
	})

	t.Run("not entitled", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, false)
		_, err := env.webPack.clt.PutJSON(env.ctx, env.endpoint("any-name"), beamDetails{})
		require.Error(t, err)
		require.ErrorContains(t, err, "not authorized to use beams")
	})
}

func TestDeleteBeam(t *testing.T) {
	t.Parallel()

	t.Run("entitled deletes beam", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, true)

		resp, err := env.webPack.clt.PostJSON(env.ctx, env.endpoint(), map[string]any{})
		require.NoError(t, err)
		var created beamDetails
		require.NoError(t, json.Unmarshal(resp.Bytes(), &created))

		_, err = env.webPack.clt.Delete(env.ctx, env.endpoint(created.Name))
		require.NoError(t, err)

		destroyReqs := env.compute.getDestroyRequests()
		require.Len(t, destroyReqs, 1)
		assert.Equal(t, created.Name, destroyReqs[0].GetBeamId())

		// The beam should no longer exist upon successful deletion.
		_, err = env.webPack.clt.Get(env.ctx, env.endpoint(created.Name), nil)
		require.Error(t, err)
		require.ErrorContains(t, err, "doesn't exist")
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, true)
		_, err := env.webPack.clt.Delete(env.ctx, env.endpoint("does-not-exist"))
		require.Error(t, err)
		require.ErrorContains(t, err, `beam "does-not-exist" doesn't exist`)
	})

	t.Run("not entitled", func(t *testing.T) {
		t.Parallel()
		env := newBeamTestEnv(t, false)
		_, err := env.webPack.clt.Delete(env.ctx, env.endpoint("any-name"))
		require.Error(t, err)
		require.ErrorContains(t, err, "not authorized to use beams")
	})
}

type beamTestEnv struct {
	ctx         context.Context
	suite       *webSuite
	webPack     *authWebPack
	clusterName string
	compute     *fakeComputeService
}

// fakeComputeService is an in-memory beam compute service client used to
// exercise the beam handlers without a real compute service.
type fakeComputeService struct {
	mu                sync.Mutex
	provisionRequests []*beamservicev1.ProvisionBeamRequest
	destroyRequests   []*beamservicev1.DestroyBeamRequest
	provisionResponse *beamservicev1.ProvisionBeamResponse
}

func (f *fakeComputeService) CreateBeam(ctx context.Context, in *beamservicev1.ProvisionBeamRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (f *fakeComputeService) WaitForBeamProvision(ctx context.Context, in *beamservicev1.WaitForBeamProvisionRequest, opts ...grpc.CallOption) (beamservicev1.BeamsOrchestratorService_WaitForBeamProvisionClient, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (f *fakeComputeService) GetInfo(ctx context.Context, in *beamservicev1.GetInfoRequest, opts ...grpc.CallOption) (*beamservicev1.GetInfoResponse, error) {
	return &beamservicev1.GetInfoResponse{}, nil
}

// ProvisionBeam fakes a beam provision request.
func (f *fakeComputeService) ProvisionBeam(_ context.Context, req *beamservicev1.ProvisionBeamRequest, _ ...grpc.CallOption) (*beamservicev1.ProvisionBeamResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.provisionRequests = append(f.provisionRequests, req)
	return f.provisionResponse, nil
}

// DestroyBeam fakes a beam destroy request.
func (f *fakeComputeService) DestroyBeam(_ context.Context, req *beamservicev1.DestroyBeamRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.destroyRequests = append(f.destroyRequests, req)
	return &emptypb.Empty{}, nil
}

// getProvisionRequests returns a copy of the provision requests made to the fake compute service.
func (f *fakeComputeService) getProvisionRequests() []*beamservicev1.ProvisionBeamRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.provisionRequests)
}

// getDestroyRequests returns a copy of the destroy requests made to the fake compute service.
func (f *fakeComputeService) getDestroyRequests() []*beamservicev1.DestroyBeamRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.destroyRequests)
}

func (e *beamTestEnv) endpoint(parts ...string) string {
	return e.webPack.clt.Endpoint(append([]string{"webapi", "sites", e.clusterName, "beams"}, parts...)...)
}

func newBeamTestEnv(t *testing.T, entitled bool, opts ...webSuiteOption) *beamTestEnv {
	t.Helper()

	// Injecting a fake compute service so the beam service is registered
	compute := &fakeComputeService{
		provisionResponse: &beamservicev1.ProvisionBeamResponse{
			SshAddr: "127.0.0.1:22",
		},
	}

	opts = append(opts,
		withModules(&modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Beams: {Enabled: entitled},
				},
			},
		}),
		withBeamsComputeClient(compute),
	)
	if entitled {
		opts = append(opts, withClusterEntitlements(map[string]*proto.EntitlementInfo{
			string(entitlements.Beams): {Enabled: true},
		}))
	}
	s := newWebSuite(t, opts...)
	createAdminUser(t, s)
	return &beamTestEnv{
		ctx:         t.Context(),
		suite:       s,
		webPack:     s.newAuthWebPack(t, "admin", skipUserCreation()),
		clusterName: s.testAuthServer.ClusterName(),
		compute:     compute,
	}
}
