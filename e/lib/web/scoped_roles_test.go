package web

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	scopedaccesscache "github.com/gravitational/teleport/lib/scopes/cache/access"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestListRootScopedRoles(t *testing.T) {
	bk, err := memory.New(memory.Config{
		Context: t.Context(),
	})
	require.NoError(t, err)
	scopedAccessService := local.NewScopedAccessService(bk)

	roles := []*scopedaccessv1.ScopedRole{
		newScopedRole("admins", "/", []string{"/**"}),
		newScopedRole("devs", "/", []string{"/dev", "/staging/**"}),
		newScopedRole("interns", "/scratch", []string{"/scratch"}),
	}
	for _, role := range roles {
		_, err := scopedAccessService.CreateScopedRole(t.Context(), scopedaccessv1.CreateScopedRoleRequest_builder{
			Role: role,
		}.Build())
		require.NoError(t, err)
	}

	aclService, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: bk,
		Modules: modulestest.EnterpriseModules(),
	})
	require.NoError(t, err)
	events := local.NewEventsService(bk)
	accessCache, err := scopedaccesscache.NewCache(scopedaccesscache.CacheConfig{
		Reader:           scopedAccessService,
		AccessListReader: aclService,
		Events:           events,
		AccessListEvents: events,
	})
	require.NoError(t, err)

	for _, tc := range []struct {
		desc        string
		query       url.Values
		expectRoles []ui.ScopedRoleListItem
	}{
		{
			desc:  "default",
			query: url.Values{},
			expectRoles: []ui.ScopedRoleListItem{
				{
					Name:  "admins",
					Scope: "/",
					AssignableScopes: []string{
						"/**",
					},
				},
				{
					Name:  "devs",
					Scope: "/",
					AssignableScopes: []string{
						"/dev",
						"/staging/**",
					},
				},
			},
		},
		{
			desc: "paged",
			query: url.Values{
				"limit": []string{"1"},
			},
			expectRoles: []ui.ScopedRoleListItem{
				{
					Name:  "admins",
					Scope: "/",
					AssignableScopes: []string{
						"/**",
					},
				},
				{
					Name:  "devs",
					Scope: "/",
					AssignableScopes: []string{
						"/dev",
						"/staging/**",
					},
				},
			},
		},
		{
			desc: "filtered",
			query: url.Values{
				"filter": []string{"adm"},
			},
			expectRoles: []ui.ScopedRoleListItem{
				{
					Name:  "admins",
					Scope: "/",
					AssignableScopes: []string{
						"/**",
					},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			var gotRoles []ui.ScopedRoleListItem
			startKey := ""
			for {
				tc.query.Set("startKey", startKey)
				resp, err := listRootScopedRoles(t.Context(), accessCache, tc.query)
				require.NoError(t, err)
				gotRoles = append(gotRoles, resp.Roles...)
				startKey = resp.StartKey
				if startKey == "" {
					break
				}
			}
			require.Equal(t, tc.expectRoles, gotRoles)
		})
	}
}

func newScopedRole(name, scope string, assignableScopes []string) *scopedaccessv1.ScopedRole {
	return scopedaccessv1.ScopedRole_builder{
		Kind:    scopedaccess.KindScopedRole,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: name,
		}.Build(),
		Scope: scope,
		Spec: scopedaccessv1.ScopedRoleSpec_builder{
			AssignableScopes: assignableScopes,
		}.Build(),
	}.Build()
}
