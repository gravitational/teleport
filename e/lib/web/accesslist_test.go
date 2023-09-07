package web

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/e/lib/web/ui"
)

func TestGetAccessLists(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	accesList1, err := accesslist.NewAccessList(header.Metadata{Name: "accesslist-1"}, accesslist.Spec{
		Title:             "access list 1",
		Audit:             accesslist.Audit{Frequency: time.Hour},
		Owners:            []accesslist.Owner{{Name: "llama", Description: "llama desc"}},
		OwnershipRequires: accesslist.Requires{Roles: []string{"admin"}},
		Grants:            accesslist.Grants{Roles: []string{"access"}},
	})
	require.NoError(t, err)
	createdAccessList1, err := authClient.AccessListClient().UpsertAccessList(context.Background(), accesList1)
	require.NoError(t, err)

	accesList2, err := accesslist.NewAccessList(header.Metadata{Name: "accesslist-2"}, accesslist.Spec{
		Title:             "access list 2",
		Audit:             accesslist.Audit{Frequency: time.Hour},
		Owners:            []accesslist.Owner{{Name: "alpaca", Description: "alpaca desc"}},
		OwnershipRequires: accesslist.Requires{Roles: []string{"admin"}},
		Grants:            accesslist.Grants{Roles: []string{"editor"}},
	})
	require.NoError(t, err)
	createdAccessList2, err := authClient.AccessListClient().UpsertAccessList(context.Background(), accesList2)
	require.NoError(t, err)

	endpoint := webPack.clt.Endpoint("enterprise", "accesslist")
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)

	var accessListResp ui.AccessListResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))
	require.Len(t, accessListResp.AccessLists, 2)
	require.ElementsMatch(t, accessListResp.AccessLists, []*accesslist.AccessList{createdAccessList1, createdAccessList2})
}

func TestCreateAccessList(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	spec := accesslist.Spec{
		Title:              "access list 1",
		Owners:             []accesslist.Owner{{Name: "llama", Description: "llama desc"}},
		OwnershipRequires:  accesslist.Requires{Roles: []string{"admin"}, Traits: trait.Traits{}},
		Grants:             accesslist.Grants{Roles: []string{"access"}, Traits: trait.Traits{}},
		MembershipRequires: accesslist.Requires{Traits: trait.Traits{}},
	}

	endpoint := webPack.clt.Endpoint("enterprise", "accesslist")
	resp, err := webPack.clt.PostJSON(s.ctx, endpoint, ui.CreateAccessListRequest{
		AuditDuration: "1h",
		Spec:          spec,
	})
	require.NoError(t, err)

	spec.Audit.Frequency = time.Hour

	var accessListResp ui.AccessListResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))
	require.Empty(t, cmp.Diff(spec, accessListResp.AccessList.Spec, cmpopts.EquateEmpty()))
	require.Equal(t, "access-list-1", accessListResp.AccessList.Metadata.Name)
}

func TestGetAccessList(t *testing.T) {
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	accesList, err := accesslist.NewAccessList(header.Metadata{Name: "accesslist-1"}, accesslist.Spec{
		Title:             "access list 1",
		Audit:             accesslist.Audit{Frequency: time.Hour},
		Owners:            []accesslist.Owner{{Name: "llama", Description: "llama desc"}},
		OwnershipRequires: accesslist.Requires{Roles: []string{"admin"}},
		Grants:            accesslist.Grants{Roles: []string{"access"}},
	})
	require.NoError(t, err)
	createdAccessList, err := authClient.AccessListClient().UpsertAccessList(context.Background(), accesList)
	require.NoError(t, err)

	endpoint := webPack.clt.Endpoint("enterprise", "accesslist", createdAccessList.GetName())
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)

	var accessListResp ui.AccessListResponse
	require.NoError(t, json.Unmarshal(resp.Bytes(), &accessListResp))

	// Unset the ID. This ID gets set just before upsert, and the "create" api does not
	// return the item with the ID updated.
	accessListResp.AccessList.Metadata.ID = 0
	require.Equal(t, createdAccessList, accessListResp.AccessList)
}
