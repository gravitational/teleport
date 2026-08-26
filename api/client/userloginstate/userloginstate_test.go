// Copyright 2023 Gravitational, Inc.
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

package userloginstate

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	userloginstatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userloginstate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/types/userloginstate"
	conv "github.com/gravitational/teleport/api/types/userloginstate/convert/v1"
	"github.com/gravitational/teleport/api/utils/clientutils"
)

type mockClient struct {
	userloginstatev1.UserLoginStateServiceClient

	t *testing.T

	getUserLoginStatesRequest       *userloginstatev1.GetUserLoginStatesRequest
	getUserLoginStateRequest        *userloginstatev1.GetUserLoginStateRequest
	upsertUserLoginStateRequest     *userloginstatev1.UpsertUserLoginStateRequest
	deleteUserLoginStateRequest     *userloginstatev1.DeleteUserLoginStateRequest
	deleteAllUserLoginStatesRequest *userloginstatev1.DeleteAllUserLoginStatesRequest
}

func (m *mockClient) GetUserLoginStates(_ context.Context, in *userloginstatev1.GetUserLoginStatesRequest, _ ...grpc.CallOption) (*userloginstatev1.GetUserLoginStatesResponse, error) {
	m.getUserLoginStatesRequest = in
	return &userloginstatev1.GetUserLoginStatesResponse{
		UserLoginStates: []*userloginstatev1.UserLoginState{
			newUserLoginStateProto(m.t, "uls1"),
			newUserLoginStateProto(m.t, "uls2"),
			newUserLoginStateProto(m.t, "uls3"),
		},
	}, nil
}

func (m *mockClient) GetUserLoginState(_ context.Context, in *userloginstatev1.GetUserLoginStateRequest, _ ...grpc.CallOption) (*userloginstatev1.UserLoginState, error) {
	m.getUserLoginStateRequest = in
	return newUserLoginStateProto(m.t, in.Name), nil
}

func (m *mockClient) UpsertUserLoginState(_ context.Context, in *userloginstatev1.UpsertUserLoginStateRequest, _ ...grpc.CallOption) (*userloginstatev1.UserLoginState, error) {
	m.upsertUserLoginStateRequest = in
	return in.UserLoginState, nil
}

func (m *mockClient) DeleteUserLoginState(_ context.Context, in *userloginstatev1.DeleteUserLoginStateRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	m.deleteUserLoginStateRequest = in
	return nil, nil
}

func (m *mockClient) DeleteAllUserLoginStates(_ context.Context, in *userloginstatev1.DeleteAllUserLoginStatesRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	m.deleteAllUserLoginStatesRequest = in
	return nil, nil
}

func (m *mockClient) ListUserLoginStates(ctx context.Context, in *userloginstatev1.ListUserLoginStatesRequest, opts ...grpc.CallOption) (*userloginstatev1.ListUserLoginStatesResponse, error) {
	if in.PageSize != 1 {
		return nil, trace.BadParameter("unsupported page size, expected 1, got %d", in.PageSize)
	}
	switch in.PageToken {
	case "", "uls1":
		return &userloginstatev1.ListUserLoginStatesResponse{
			UserLoginStates: []*userloginstatev1.UserLoginState{newUserLoginStateProto(m.t, "uls1")},
			NextPageToken:   "uls2",
		}, nil
	case "uls2":
		return &userloginstatev1.ListUserLoginStatesResponse{
			UserLoginStates: []*userloginstatev1.UserLoginState{newUserLoginStateProto(m.t, "uls2")},
			NextPageToken:   "uls3",
		}, nil
	case "uls3":
		return &userloginstatev1.ListUserLoginStatesResponse{
			UserLoginStates: []*userloginstatev1.UserLoginState{newUserLoginStateProto(m.t, "uls3")},
			NextPageToken:   "",
		}, nil
	}
	return nil, trace.BadParameter("unsupported page token")
}

func TestGetListUserLoginStates(t *testing.T) {
	t.Parallel()
	mockClient := &mockClient{t: t}
	client := NewClient(mockClient)

	states, err := client.GetUserLoginStates(context.Background())
	require.NoError(t, err)

	require.NotNil(t, mockClient.getUserLoginStatesRequest)

	require.Empty(t, cmp.Diff([]*userloginstate.UserLoginState{
		newUserLoginState(t, "uls1"),
		newUserLoginState(t, "uls2"),
		newUserLoginState(t, "uls3"),
	}, states))

	t.Run("test list user login state with pagination", func(t *testing.T) {
		var items []*userloginstate.UserLoginState
		for item, err := range clientutils.ResourcesWithPageSize(context.Background(), client.ListUserLoginStates, 1) {
			require.NoError(t, err)
			items = append(items, item)
		}

		require.Empty(t, cmp.Diff([]*userloginstate.UserLoginState{
			newUserLoginState(t, "uls1"),
			newUserLoginState(t, "uls2"),
			newUserLoginState(t, "uls3"),
		}, items))
	})
}

func TestGetUserLoginState(t *testing.T) {
	t.Parallel()

	mockClient := &mockClient{t: t}
	client := NewClient(mockClient)

	uls, err := client.GetUserLoginState(context.Background(), "uls1")
	require.NoError(t, err)

	require.Equal(t, "uls1", mockClient.getUserLoginStateRequest.Name)

	require.Empty(t, cmp.Diff(newUserLoginState(t, "uls1"), uls))
}

func TestUpsertUserLoginState(t *testing.T) {
	t.Parallel()

	mockClient := &mockClient{t: t}
	client := NewClient(mockClient)

	uls := newUserLoginState(t, "uls1")

	resp, err := client.UpsertUserLoginState(context.Background(), uls)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(uls, mustFromProto(t, mockClient.upsertUserLoginStateRequest.UserLoginState)))
	require.Empty(t, cmp.Diff(resp, newUserLoginState(t, "uls1")))
}

func TestDeleteUserLoginState(t *testing.T) {
	t.Parallel()

	mockClient := &mockClient{t: t}
	client := NewClient(mockClient)

	require.NoError(t, client.DeleteUserLoginState(context.Background(), "uls1"))

	require.Equal(t, "uls1", mockClient.deleteUserLoginStateRequest.Name)
}

func TestDeleteAllUserLoginStates(t *testing.T) {
	t.Parallel()

	mockClient := &mockClient{t: t}
	client := NewClient(mockClient)

	require.NoError(t, client.DeleteAllUserLoginStates(context.Background()))

	require.NotNil(t, mockClient.deleteAllUserLoginStatesRequest)
}

func newUserLoginStateProto(t *testing.T, name string) *userloginstatev1.UserLoginState {
	t.Helper()

	return conv.ToProto(newUserLoginState(t, name))
}

func newUserLoginState(t *testing.T, name string) *userloginstate.UserLoginState {
	t.Helper()

	uls, err := userloginstate.New(header.Metadata{
		Name: name,
	}, userloginstate.Spec{
		Roles:          []string{"role1", "role2"},
		OriginalTraits: trait.Traits{},
		Traits: trait.Traits{
			"trait1": []string{"value1", "value2"},
			"trait2": []string{"value1", "value2"},
		},
		UserType: types.UserTypeLocal,
	})
	require.NoError(t, err)

	return uls
}

func mustFromProto(t *testing.T, msg *userloginstatev1.UserLoginState) *userloginstate.UserLoginState {
	t.Helper()

	uls, err := conv.FromProto(msg)
	require.NoError(t, err)

	return uls
}
