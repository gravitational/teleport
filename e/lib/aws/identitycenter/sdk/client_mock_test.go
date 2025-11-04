package sdk

import (
	"context"
	"testing"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestClientMock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var c Client = NewClientMock(nil /* custom mock data */)
	t.Run("should list groups", func(t *testing.T) {
		resp, err := c.ListGroups(ctx)
		require.NoError(t, err)
		require.Len(t, resp, 2)
	})

	t.Run("should list transitive user account assignments", func(t *testing.T) {
		// Asserts that the mock client returns both directly-assigned account
		// assignments and assignments granted through group membership for a
		// given user
		asmts, err := c.ListAssignments(ctx, "user1", ssoadmintypes.PrincipalTypeUser)
		require.NoError(t, err)

		expected := []*Assignment{
			{
				PrincipalType:    ssoadmintypes.PrincipalTypeUser,
				AccountID:        "1111111111",
				PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
			},
			{
				PrincipalType:    ssoadmintypes.PrincipalTypeUser,
				AccountID:        "2222222222",
				PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
			},
			{
				PrincipalType:    ssoadmintypes.PrincipalTypeGroup,
				AccountID:        "1111111111",
				PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
			},
		}
		require.ElementsMatch(t, expected, asmts)
	})

	t.Run("should list group account assignments", func(t *testing.T) {
		asmts, err := c.ListAssignments(ctx, "group2", ssoadmintypes.PrincipalTypeGroup)
		require.NoError(t, err)

		expected := []*Assignment{
			{
				PrincipalType:    ssoadmintypes.PrincipalTypeGroup,
				AccountID:        "2222222222",
				PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
			},
		}
		require.ElementsMatch(t, expected, asmts)
	})
}

func TestAccountAssignmentMock(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var c Client = NewClientMock(nil /* custom mock data */)

	want := []*UserWithAssignment{
		{
			User: &User{ID: "user1", UserName: "user_one"},
			Assignments: []*Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"},
				{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
			},
		},
		{
			User: &User{ID: "user2", UserName: "user_two"},
			Assignments: []*Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
			},
		},
	}
	assertUsersAssignments(t, c, want)

	t.Run("should delete user account assignment", func(t *testing.T) {
		deleteResp, err := c.DeleteAccountAssignment(ctx, &DeleteAccountAssignmentRequest{
			PrincipalID:      "user1",
			PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
			PrincipalType:    ssoadmintypes.PrincipalTypeUser,
			AccountID:        "1111111111",
		})
		require.NoError(t, err)
		require.NoError(t, c.WaitForDeleteAccountAssignmentResult(ctx, deleteResp.RequestID))

		deleteResp, err = c.DeleteAccountAssignment(ctx, &DeleteAccountAssignmentRequest{
			PrincipalID:      "user2",
			PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
			PrincipalType:    ssoadmintypes.PrincipalTypeUser,
			AccountID:        "1111111111",
		})
		require.NoError(t, err)
		require.NoError(t, c.WaitForDeleteAccountAssignmentResult(ctx, deleteResp.RequestID))

		want := []*UserWithAssignment{
			{
				User: &User{ID: "user1", UserName: "user_one"},
				Assignments: []*Assignment{
					{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
				},
			},
			{
				User:        &User{ID: "user2", UserName: "user_two"},
				Assignments: []*Assignment{},
			},
		}
		assertUsersAssignments(t, c, want)

	})

	t.Run("should create user account assignment", func(t *testing.T) {
		createResp, err := c.CreateAccountAssignment(ctx, &CreateAccountAssignmentRequest{
			PrincipalID:      "user2",
			PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
			PrincipalType:    ssoadmintypes.PrincipalTypeUser,
			AccountID:        "1111111111",
		})
		require.NoError(t, err)
		require.NoError(t, c.WaitForCreateAccountAssignmentResult(ctx, createResp.RequestID))

		want := []*UserWithAssignment{
			{
				User: &User{ID: "user1", UserName: "user_one"},
				Assignments: []*Assignment{
					{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
				},
			},
			{
				User: &User{ID: "user2", UserName: "user_two"},
				Assignments: []*Assignment{
					{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly", PrincipalType: "USER"},
				},
			},
		}
		assertUsersAssignments(t, c, want)
	})

	t.Run("account assignment failures", func(t *testing.T) {
		func(subtestT *testing.T, client *ClientMock) {
			client.MonkeyPatch.WaitForCreateAccountAssignmentResult = func(_ context.Context, _ string) error {
				return trace.Errorf("account assignment creation failed: some error")
			}
			client.MonkeyPatch.WaitForDeleteAccountAssignmentResult = func(_ context.Context, _ string) error {
				return trace.Errorf("account assignment deletion failed: some error")
			}
			subtestT.Cleanup(func() {
				client.MonkeyPatch.WaitForCreateAccountAssignmentResult = nil
				client.MonkeyPatch.WaitForDeleteAccountAssignmentResult = nil
			})
		}(t, c.(*ClientMock))

		createResp, err := c.CreateAccountAssignment(ctx, &CreateAccountAssignmentRequest{
			PrincipalID:      "user333333345",
			PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
			PrincipalType:    ssoadmintypes.PrincipalTypeUser,
			AccountID:        "1111111111",
		})
		require.NoError(t, err)
		require.Error(t, c.WaitForCreateAccountAssignmentResult(ctx, createResp.RequestID))

		deleteResp, err := c.DeleteAccountAssignment(ctx, &DeleteAccountAssignmentRequest{
			PrincipalID:      "user333333345",
			PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
			PrincipalType:    ssoadmintypes.PrincipalTypeUser,
			AccountID:        "1111111111",
		})
		require.NoError(t, err)
		require.Error(t, c.WaitForDeleteAccountAssignmentResult(ctx, deleteResp.RequestID))
	})
}

func assertUsersAssignments(t *testing.T, c Client, want []*UserWithAssignment) {
	listResp, err := listUsersWithAccountAndPermAssignment(context.Background(), c)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(listResp, want))
}

// listUsersWithAccountAndPermAssignment lists Identity Center users with assigned accounts and permission sets.
func listUsersWithAccountAndPermAssignment(ctx context.Context, c Client) ([]*UserWithAssignment, error) {
	users, err := c.ListUsers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]*UserWithAssignment, 0, len(users))
	for _, v := range users {
		assignments, err := c.ListUserAssignments(ctx, v.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		out = append(out, &UserWithAssignment{
			User:        v,
			Assignments: assignments,
		})
	}
	return out, nil
}
