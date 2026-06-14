// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package accesslist

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestClientPreservesDisplayFields(t *testing.T) {
	t.Parallel()

	client := newAccessListClientForTesting(t)

	accessList, err := client.GetAccessList(t.Context(), "access-list")
	require.NoError(t, err)
	require.Equal(t, types.UserDisplay{
		Primary:   "Owner One",
		Secondary: "owner@example.com",
	}, accessList.Spec.Owners[0].Display)

	members, nextToken, err := client.ListAccessListMembers(t.Context(), "access-list", 100, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Equal(t, types.UserDisplay{
		Primary:   "Member One",
		Secondary: "member@example.com",
	}, members[0].Spec.Display)
	require.Equal(t, types.UserDisplay{
		Primary:   "Adder One",
		Secondary: "adder@example.com",
	}, members[0].Spec.AddedByDisplay)
}

func newAccessListClientForTesting(t *testing.T) *Client {
	t.Helper()

	lis := bufconn.Listen(1024)
	t.Cleanup(func() {
		assert.NoError(t, lis.Close())
	})

	server := grpc.NewServer()
	accesslistv1.RegisterAccessListServiceServer(server, &fakeAccessListService{})

	var wg sync.WaitGroup
	wg.Go(func() {
		assert.NoError(t, server.Serve(lis))
	})
	t.Cleanup(func() {
		server.GracefulStop()
		wg.Wait()
	})

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, conn.Close())
	})

	return NewClient(accesslistv1.NewAccessListServiceClient(conn))
}

type fakeAccessListService struct {
	accesslistv1.UnimplementedAccessListServiceServer
}

func (fakeAccessListService) GetAccessList(context.Context, *accesslistv1.GetAccessListRequest) (*accesslistv1.AccessList, error) {
	return &accesslistv1.AccessList{
		Header: accessListHeader(types.KindAccessList, "access-list"),
		Spec: &accesslistv1.AccessListSpec{
			Title:              "Access List",
			MembershipRequires: &accesslistv1.AccessListRequires{},
			OwnershipRequires:  &accesslistv1.AccessListRequires{},
			Owners: []*accesslistv1.AccessListOwner{
				{
					Name: "owner",
					Display: &accesslistv1.UserDisplay{
						Primary:   "Owner One",
						Secondary: "owner@example.com",
					},
				},
			},
			Audit: &accesslistv1.AccessListAudit{},
		},
	}, nil
}

func (fakeAccessListService) ListAccessListMembers(context.Context, *accesslistv1.ListAccessListMembersRequest) (*accesslistv1.ListAccessListMembersResponse, error) {
	return &accesslistv1.ListAccessListMembersResponse{
		Members: []*accesslistv1.Member{
			{
				Header: accessListHeader(types.KindAccessListMember, "access-list/member"),
				Spec: &accesslistv1.MemberSpec{
					AccessList: "access-list",
					Name:       "member",
					AddedBy:    "adder",
					Display: &accesslistv1.UserDisplay{
						Primary:   "Member One",
						Secondary: "member@example.com",
					},
					AddedByDisplay: &accesslistv1.UserDisplay{
						Primary:   "Adder One",
						Secondary: "adder@example.com",
					},
				},
			},
		},
	}, nil
}

func accessListHeader(kind, name string) *headerv1.ResourceHeader {
	return &headerv1.ResourceHeader{
		Kind:    kind,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
	}
}
