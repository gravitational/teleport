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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
)

// testUserDisplays is the proto display map the fake service returns; "plain"
// is a live user with no distinct display (present but empty).
var testUserDisplays = map[string]*accesslistv1.UserDisplay{
	"owner": {Primary: "Owner One", Secondary: "owner@example.com"},
	"plain": {},
}

// wantUserDisplays is testUserDisplays after proto->Go conversion.
var wantUserDisplays = map[string]types.UserDisplay{
	"owner": {Primary: "Owner One", Secondary: "owner@example.com"},
	"plain": {},
}

func TestListAccessListMembersWithDisplays(t *testing.T) {
	t.Parallel()

	client := newAccessListClientForTesting(t, &fakeAccessListService{})

	members, displays, nextToken, err := client.ListAccessListMembersWithDisplays(t.Context(), "access-list", 100, "")

	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Len(t, members, 1)
	require.Equal(t, "access-list/member", members[0].GetName())
	require.Equal(t, wantUserDisplays, displays)
}

func TestGetAccessListWithDisplays(t *testing.T) {
	t.Parallel()

	client := newAccessListClientForTesting(t, &fakeAccessListService{})

	accessList, displays, err := client.GetAccessListWithDisplays(t.Context(), "access-list")
	require.NoError(t, err)
	require.Equal(t, "access-list", accessList.GetName())
	require.Equal(t, wantUserDisplays, displays)
}

func TestGetAccessListWithDisplaysFallback(t *testing.T) {
	t.Parallel()

	// Old control planes don't implement GetAccessListV2: fall back to
	// GetAccessList with no displays.
	client := newAccessListClientForTesting(t, &fakeAccessListService{v2Unimplemented: true})

	accessList, displays, err := client.GetAccessListWithDisplays(t.Context(), "access-list")
	require.NoError(t, err)
	require.Equal(t, "access-list", accessList.GetName())
	require.Empty(t, displays)
}

func newAccessListClientForTesting(t *testing.T, svc *fakeAccessListService) *Client {
	t.Helper()

	lis := bufconn.Listen(1024)
	t.Cleanup(func() {
		assert.NoError(t, lis.Close())
	})

	server := grpc.NewServer()
	accesslistv1.RegisterAccessListServiceServer(server, svc)

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
		// Convert gRPC status errors into trace errors like the real client
		// connection does, so fallbacks see trace.NotImplementedError.
		grpc.WithChainUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, conn.Close())
	})

	return NewClient(accesslistv1.NewAccessListServiceClient(conn))
}

type fakeAccessListService struct {
	accesslistv1.UnimplementedAccessListServiceServer

	v2Unimplemented bool
}

func (f *fakeAccessListService) GetAccessList(_ context.Context, req *accesslistv1.GetAccessListRequest) (*accesslistv1.AccessList, error) {
	return testAccessList(req.GetName()), nil
}

func (f *fakeAccessListService) GetAccessListV2(_ context.Context, req *accesslistv1.GetAccessListV2Request) (*accesslistv1.GetAccessListV2Response, error) {
	if f.v2Unimplemented {
		return nil, status.Error(codes.Unimplemented, "unknown method GetAccessListV2")
	}
	return &accesslistv1.GetAccessListV2Response{
		AccessList:    testAccessList(req.GetName()),
		OwnerDisplays: testUserDisplays,
	}, nil
}

func (f *fakeAccessListService) ListAccessListMembers(_ context.Context, req *accesslistv1.ListAccessListMembersRequest) (*accesslistv1.ListAccessListMembersResponse, error) {
	resp := &accesslistv1.ListAccessListMembersResponse{
		Members: []*accesslistv1.Member{
			{
				Header: accessListHeader(types.KindAccessListMember, "access-list/member"),
				Spec: &accesslistv1.MemberSpec{
					AccessList: req.GetAccessList(),
					Name:       "member",
					AddedBy:    "adder",
				},
			},
		},
	}
	if req.GetIncludeUserDisplays() {
		resp.UserDisplays = testUserDisplays
	}
	return resp, nil
}

func testAccessList(name string) *accesslistv1.AccessList {
	return &accesslistv1.AccessList{
		Header: accessListHeader(types.KindAccessList, name),
		Spec: &accesslistv1.AccessListSpec{
			Title:              "Access List",
			MembershipRequires: &accesslistv1.AccessListRequires{},
			OwnershipRequires:  &accesslistv1.AccessListRequires{},
			Owners: []*accesslistv1.AccessListOwner{
				{Name: "owner"},
				{Name: "plain"},
			},
			Audit: &accesslistv1.AccessListAudit{},
		},
	}
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
