package iter

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	ictest "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
)

type rolePage struct {
	data *proto.ListRolesResponse
	err  error
}

type mockRoleLister struct {
	t               *testing.T
	expectedPageKey string
	pages           map[string]rolePage
}

func (m *mockRoleLister) ListRoles(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error) {
	require.Equal(m.t, m.expectedPageKey, req.StartKey)

	page, ok := m.pages[req.StartKey]
	require.True(m.t, ok, "invalid page key: %q", req.StartKey)
	if page.data != nil {
		m.expectedPageKey = page.data.NextKey
	}

	return page.data, page.err
}

func TestAllAccountAssignmentRoles(t *testing.T) {
	type rolePair struct {
		role *types.RoleV6
		err  error
	}

	aaroles := make([]*types.RoleV6, 10)
	for i := range aaroles {
		aaroles[i] = ictest.AccountAssignmentRole{
			Name:             fmt.Sprintf("Account Assignment Role #%02.d", i),
			AccountID:        "123456789",
			PermissionSetARN: "*",
		}.Build(t)
	}

	tmpRole, err := types.NewRole("not-an-aa-role", types.RoleSpecV6{})
	require.NoError(t, err)
	nonAARole, ok := tmpRole.(*types.RoleV6)
	require.True(t, ok, "Expected RoleV6, got %T", tmpRole)

	someError := errors.New("oops")

	testCases := []struct {
		name             string
		pages            map[string]rolePage
		expectedSequence []rolePair
	}{
		{
			name: "empty",
			pages: map[string]rolePage{
				"": {
					data: &proto.ListRolesResponse{NextKey: ""},
				},
			},
		},
		{
			name: "simple error",
			pages: map[string]rolePage{
				"": {
					err: someError,
				},
			},
			expectedSequence: []rolePair{
				{nil, someError},
			},
		},
		{
			name: "single page",
			pages: map[string]rolePage{
				"": {
					data: &proto.ListRolesResponse{
						Roles:   aaroles[0:5],
						NextKey: "",
					},
				},
			},
			expectedSequence: []rolePair{
				{aaroles[0], nil},
				{aaroles[1], nil},
				{aaroles[2], nil},
				{aaroles[3], nil},
				{aaroles[4], nil},
			},
		},
		{
			name: "non-account-assignment roles are not listed",
			pages: map[string]rolePage{
				"": {
					data: &proto.ListRolesResponse{
						Roles: []*types.RoleV6{
							aaroles[0],
							aaroles[2],
							aaroles[3],
							nonAARole,
							aaroles[4],
						},
						NextKey: "",
					},
				},
			},
			expectedSequence: []rolePair{
				{aaroles[0], nil},
				{aaroles[2], nil},
				{aaroles[3], nil},
				{aaroles[4], nil},
			},
		},
		{
			name: "multipage",
			pages: map[string]rolePage{
				"": {
					data: &proto.ListRolesResponse{
						Roles:   aaroles[0:5],
						NextKey: "two",
					},
				},
				"two": {
					data: &proto.ListRolesResponse{
						Roles:   aaroles[5:],
						NextKey: "",
					},
				},
			},
			expectedSequence: []rolePair{
				{aaroles[0], nil},
				{aaroles[1], nil},
				{aaroles[2], nil},
				{aaroles[3], nil},
				{aaroles[4], nil},
				{aaroles[5], nil},
				{aaroles[6], nil},
				{aaroles[7], nil},
				{aaroles[8], nil},
				{aaroles[9], nil},
			},
		},
		{
			name: "iteration stops on error",
			pages: map[string]rolePage{
				"": {
					data: &proto.ListRolesResponse{
						Roles:   aaroles[0:1],
						NextKey: "two",
					},
				},
				"two": {
					data: &proto.ListRolesResponse{
						Roles:   aaroles[1:2],
						NextKey: "three",
					},
					err: someError,
				},
				"three": {
					data: &proto.ListRolesResponse{
						Roles:   aaroles[2:],
						NextKey: "",
					},
				},
			},
			expectedSequence: []rolePair{
				{aaroles[0], nil},
				{nil, someError},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			mockSvc := &mockRoleLister{
				t:     t,
				pages: test.pages,
			}

			var actualSequence []rolePair
			for role, err := range AllAccountAssignmentRoles(context.Background(), mockSvc) {
				actualSequence = append(actualSequence, rolePair{role, err})
			}

			require.Len(t, actualSequence, len(test.expectedSequence))
			for i, expected := range test.expectedSequence {
				actual := actualSequence[i]

				require.Equal(t, expected.role, actual.role)
				if expected.err == nil {
					require.NoError(t, actual.err)
				} else {
					require.ErrorIs(t, actual.err, expected.err)
				}
			}
		})
	}
}
