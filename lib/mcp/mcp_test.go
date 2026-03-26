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

package mcp

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/lib/utils/mcptest"
)

type mcpTestEnv struct {
	ctx        context.Context
	cancel     context.CancelFunc
	authClient authclient.ClientI
	mcpClient  *mcpclient.Client
}

func newMCPTestEnv(t *testing.T) *mcpTestEnv {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		ClusterName: "mcp.test",
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	tlsServer, err := authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tlsServer.Close()) })

	authClient, err := tlsServer.NewClient(authtest.TestAdmin())
	require.NoError(t, err)

	wrappedAuthClient := newWrappedAuthClient(authClient)

	mcpServer, err := NewMCPServer(Config{
		Auth:         wrappedAuthClient,
		WebProxyAddr: "mcp.test",
		Log:          logtest.NewLogger(),
	})
	require.NoError(t, err)

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
	})

	go mcpServer.ListenStdio(ctx, serverConn, serverConn)

	mcpClient := mcptest.NewStdioClient(t, clientConn, clientConn)
	mcptest.MustInitializeClient(t, mcpClient)

	return &mcpTestEnv{
		ctx:        ctx,
		cancel:     cancel,
		authClient: wrappedAuthClient,
		mcpClient:  mcpClient,
	}
}

// wrappedAuthClient wraps an auth client with an override to a mock access
// lists client since auth server in open-source Teleport does implement
// access lists service.
type wrappedAuthClient struct {
	authclient.ClientI
	accessLists *mockAccessListClient
}

func newWrappedAuthClient(authClient authclient.ClientI) *wrappedAuthClient {
	return &wrappedAuthClient{
		ClientI:     authClient,
		accessLists: newMockAccessListClient(),
	}
}

func (m *wrappedAuthClient) AccessListClient() services.AccessLists {
	return m.accessLists
}

type mockAccessList struct {
	accessList *accesslist.AccessList
	members    map[string]*accesslist.AccessListMember
	reviews    []*accesslist.Review
}

type mockAccessListClient struct {
	services.AccessLists
	lists map[string]*mockAccessList
}

func newMockAccessListClient() *mockAccessListClient {
	return &mockAccessListClient{
		lists: make(map[string]*mockAccessList),
	}
}

func (m *mockAccessListClient) GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error) {
	var lists []*accesslist.AccessList
	for _, list := range m.lists {
		lists = append(lists, list.accessList)
	}
	return lists, nil
}

func (m *mockAccessListClient) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	list, ok := m.lists[name]
	if !ok {
		return nil, trace.NotFound("access list %q not found", name)
	}
	return list.accessList, nil
}

func (m *mockAccessListClient) GetAccessListsToReview(ctx context.Context) ([]*accesslist.AccessList, error) {
	var lists []*accesslist.AccessList
	for _, list := range m.lists {
		lists = append(lists, list.accessList)
	}
	return lists, nil
}

func (m *mockAccessListClient) UpsertAccessList(ctx context.Context, list *accesslist.AccessList) (*accesslist.AccessList, error) {
	if _, ok := m.lists[list.GetName()]; ok {
		m.lists[list.GetName()].accessList = list
	} else {
		m.lists[list.GetName()] = &mockAccessList{
			accessList: list,
			members:    make(map[string]*accesslist.AccessListMember),
			reviews:    []*accesslist.Review{},
		}
	}
	return list, nil
}

func (m *mockAccessListClient) UpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	_, ok := m.lists[member.Spec.AccessList]
	if !ok {
		return nil, trace.NotFound("access list %q not found", member.Spec.AccessList)
	}
	m.lists[member.Spec.AccessList].members[member.GetName()] = member
	return member, nil
}

func (m *mockAccessListClient) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
	list, ok := m.lists[accessListName]
	if !ok {
		return nil, "", trace.NotFound("access list %q not found", accessListName)
	}
	var members []*accesslist.AccessListMember
	for _, member := range list.members {
		members = append(members, member)
	}
	return members, "", nil
}

func (m *mockAccessListClient) CreateAccessListReview(ctx context.Context, review *accesslist.Review) (*accesslist.Review, time.Time, error) {
	list, ok := m.lists[review.Spec.AccessList]
	if !ok {
		return nil, time.Time{}, trace.NotFound("access list %q not found", review.Spec.AccessList)
	}
	list.reviews = append(list.reviews, review)
	return review, time.Time{}, nil
}

func (m *mockAccessListClient) ListAccessListReviews(ctx context.Context, accessListName string, pageSize int, pageToken string) ([]*accesslist.Review, string, error) {
	list, ok := m.lists[accessListName]
	if !ok {
		return nil, "", trace.NotFound("access list %q not found", accessListName)
	}
	return list.reviews, "", nil
}
