package web

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestListBeams(t *testing.T) {
	// Required to register the beam grpc service
	t.Setenv("TELEPORT_BEAM_SERVICE_ADDRESS", "something")

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

			ctx := t.Context()
			s := newWebSuite(
				t,
				withWebPackAuthCacheEnabled(enableCache),
				withModules(&modulestest.Modules{
					TestBuildType: modules.BuildEnterprise,
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.Beams: {Enabled: true},
						},
					},
				}),
			)

			createAdminUser(t, s)

			webPack := s.newAuthWebPack(t, "admin", skipUserCreation())
			clusterName := s.testAuthServer.ClusterName()

			b := createBeam(t, s, "1")

			endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "beams")
			resp, err := webPack.clt.Get(ctx, endpoint, nil)
			require.NoError(t, err)

			var got listBeamsResponse
			err = json.Unmarshal(resp.Bytes(), &got)
			require.NoError(t, err)
			assert.Len(t, got.Items, 1)
			assert.Empty(t, cmp.Diff(got.Items, []beam{
				{
					Name:    b.GetMetadata().GetName(),
					Alias:   b.GetStatus().GetAlias(),
					User:    b.GetStatus().GetUser(),
					Expires: b.GetSpec().GetExpires().AsTime(),
					NodeId:  b.GetStatus().GetNodeId(),
					AppName: b.GetStatus().GetAppName(),
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

			ctx := t.Context()
			s := newWebSuite(
				t,
				withWebPackAuthCacheEnabled(enableCache),
				withModules(&modulestest.Modules{
					TestBuildType: modules.BuildEnterprise,
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.Beams: {Enabled: true},
						},
					},
				}),
			)

			createAdminUser(t, s)

			webPack := s.newAuthWebPack(t, "admin", skipUserCreation())
			clusterName := s.testAuthServer.ClusterName()

			createBeam(t, s, "1")
			b2 := createBeam(t, s, "2")

			endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "beams")
			resp, err := webPack.clt.Get(ctx, endpoint, url.Values{
				"user": []string{"user-2@example.com"},
			})
			require.NoError(t, err)

			var got listBeamsResponse
			err = json.Unmarshal(resp.Bytes(), &got)
			require.NoError(t, err)
			assert.Len(t, got.Items, 1)
			assert.Empty(t, cmp.Diff(got.Items, []beam{
				{
					Name:    b2.GetMetadata().GetName(),
					Alias:   b2.GetStatus().GetAlias(),
					User:    b2.GetStatus().GetUser(),
					Expires: b2.GetSpec().GetExpires().AsTime(),
					NodeId:  b2.GetStatus().GetNodeId(),
					AppName: b2.GetStatus().GetAppName(),
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

			ctx := t.Context()
			s := newWebSuite(
				t,
				withWebPackAuthCacheEnabled(enableCache),
				withModules(&modulestest.Modules{
					TestBuildType: modules.BuildEnterprise,
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.Beams: {Enabled: true},
						},
					},
				}),
			)

			createAdminUser(t, s)

			webPack := s.newAuthWebPack(t, "admin", skipUserCreation())
			clusterName := s.testAuthServer.ClusterName()

			b1 := createBeam(t, s, "1")
			b2 := createBeam(t, s, "2")
			b3 := createBeam(t, s, "3")

			endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "beams")

			resp, err := webPack.clt.Get(ctx, endpoint, url.Values{
				"page_size":  []string{"2"},
				"sort_field": []string{"name"},
				"sort_dir":   []string{"asc"},
			})
			require.NoError(t, err)

			var firstPage listBeamsResponse
			err = json.Unmarshal(resp.Bytes(), &firstPage)
			require.NoError(t, err)
			require.Len(t, firstPage.Items, 2)
			require.NotEmpty(t, firstPage.NextPageToken)
			assert.Equal(t, []beam{
				{
					Name:    b1.GetMetadata().GetName(),
					Alias:   b1.GetStatus().GetAlias(),
					User:    b1.GetStatus().GetUser(),
					Expires: b1.GetSpec().GetExpires().AsTime(),
					NodeId:  b1.GetStatus().GetNodeId(),
					AppName: b1.GetStatus().GetAppName(),
				},
				{
					Name:    b2.GetMetadata().GetName(),
					Alias:   b2.GetStatus().GetAlias(),
					User:    b2.GetStatus().GetUser(),
					Expires: b2.GetSpec().GetExpires().AsTime(),
					NodeId:  b2.GetStatus().GetNodeId(),
					AppName: b2.GetStatus().GetAppName(),
				},
			}, firstPage.Items)

			resp, err = webPack.clt.Get(ctx, endpoint, url.Values{
				"page_size":  []string{"2"},
				"page_token": []string{firstPage.NextPageToken},
				"sort_field": []string{"name"},
				"sort_dir":   []string{"asc"},
			})
			require.NoError(t, err)

			var secondPage listBeamsResponse
			err = json.Unmarshal(resp.Bytes(), &secondPage)
			require.NoError(t, err)
			assert.Equal(t, []beam{
				{
					Name:    b3.GetMetadata().GetName(),
					Alias:   b3.GetStatus().GetAlias(),
					User:    b3.GetStatus().GetUser(),
					Expires: b3.GetSpec().GetExpires().AsTime(),
					NodeId:  b3.GetStatus().GetNodeId(),
					AppName: b3.GetStatus().GetAppName(),
				},
			}, secondPage.Items)
			assert.Empty(t, secondPage.NextPageToken)
		})
	}
}

func testListBeamsWithSorting(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	s := newWebSuite(
		t,
		withWebPackAuthCacheEnabled(true),
		withModules(&modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Beams: {Enabled: true},
				},
			},
		}),
	)

	createAdminUser(t, s)

	webPack := s.newAuthWebPack(t, "admin", skipUserCreation())
	clusterName := s.testAuthServer.ClusterName()

	expires := time.Now().Add(24 * time.Hour)

	b1 := createBeam(t, s, "1", func(beam *beamsv1.Beam) {
		beam.Metadata.Name = "222"
		beam.Status.Alias = "beam-alpha"
		beam.Status.User = "user-3@example.com"
		beam.Spec.Expires = timestamppb.New(expires.Add(3 * time.Millisecond))
	})
	b2 := createBeam(t, s, "2", func(beam *beamsv1.Beam) {
		beam.Metadata.Name = "333"
		beam.Status.Alias = "beam-charlie"
		beam.Status.User = "user-2@example.com"
		beam.Spec.Expires = timestamppb.New(expires.Add(1 * time.Millisecond))
	})
	b3 := createBeam(t, s, "3", func(beam *beamsv1.Beam) {
		beam.Metadata.Name = "111"
		beam.Status.Alias = "beam-beta"
		beam.Status.User = "user-1@example.com"
		beam.Spec.Expires = timestamppb.New(expires.Add(2 * time.Millisecond))
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
				b2.Spec.Expires.AsTime(),
				b3.Spec.Expires.AsTime(),
				b1.Spec.Expires.AsTime(),
			},
		},
		{
			name:  "by-expires-desc",
			field: "expires",
			dir:   "desc",
			ordered: []any{
				b1.Spec.Expires.AsTime(),
				b3.Spec.Expires.AsTime(),
				b2.Spec.Expires.AsTime(),
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

			endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "beams")
			resp, err := webPack.clt.Get(ctx, endpoint, url.Values{
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

	ctx := t.Context()
	s := newWebSuite(
		t,
		withModules(&modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.Beams: {Enabled: false},
				},
			},
		}),
	)

	createAdminUser(t, s)

	webPack := s.newAuthWebPack(t, "admin", skipUserCreation())
	clusterName := s.testAuthServer.ClusterName()

	// known beams path
	{
		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "beams")
		_, err := webPack.clt.Get(ctx, endpoint, nil)
		require.Error(t, err)
		require.ErrorContains(t, err, "not authorized to use beams", endpoint)
	}

	// unknown beams path
	{
		endpoint := webPack.clt.Endpoint("webapi", "sites", clusterName, "beams", "unknown")
		_, err := webPack.clt.Get(ctx, endpoint, nil)
		require.Error(t, err)
		require.ErrorContains(t, err, "path not found", endpoint)
	}
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

	pre := &beamsv1.Beam{
		Kind:    "beam",
		Version: "v1",
		Metadata: &headerv1.Metadata{
			Name:    name,
			Expires: timestamppb.New(expiresMeta),
			Labels: map[string]string{
				types.BeamIDLabel:    name,
				types.BeamOwnerLabel: user,
				types.BeamAliasLabel: alias,
			},
		},
		Spec: &beamsv1.BeamSpec{
			AllowedDomains: []string{},
			Expires:        timestamppb.New(expiresSpec),
			Egress:         beamsv1.EgressMode_EGRESS_MODE_UNRESTRICTED,
		},
		Status: &beamsv1.BeamStatus{
			User:    user,
			Alias:   alias,
			NodeId:  fmt.Sprintf("beam-%s-node", id),
			AppName: fmt.Sprintf("beam-%s-app", id),
		},
	}

	for _, opt := range options {
		opt(pre)
	}

	actions, err := s.testAuthServer.AuthServer.AuthServer.AppendPutBeamActions(nil, pre, backend.NotExists())
	require.NoError(t, err)

	_, err = s.testAuthServer.AuthServer.Backend.AtomicWrite(ctx, actions)
	require.NoError(t, err)

	// Wait for changes to propagate to the cache
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		_, err := s.testAuthServer.AuthServer.AuthServer.GetBeam(ctx, pre.Metadata.Name)
		require.NoError(collect, err)
	}, 10*time.Second, 100*time.Millisecond)

	beam, err := s.testAuthServer.AuthServer.AuthServer.GetBeam(ctx, pre.Metadata.Name)
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
