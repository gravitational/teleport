package sdk

import (
	"context"
	"testing"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestClientMock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var c Client = NewClientMock(nil /* custom mock data */)
	t.Run("should list users with account and permission assignments", func(t *testing.T) {
		resp, err := c.ListGroupsWithAccountAndPermAssignment(ctx)
		require.NoError(t, err)
		require.Len(t, resp, 2)
	})

	t.Run("should list groups with account and permission assignments", func(t *testing.T) {
		resp, err := c.ListUsersWithAccountAndPermAssignment(ctx)
		require.NoError(t, err)
		require.Len(t, resp, 2)
	})

}

func TestAccountAssigmentMock(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var c Client = NewClientMock(nil /* custom mock data */)

	want := []*UserWithAssignment{
		{
			User: &User{ID: "user1", UserName: "user_one"},
			Assignments: []*Assigment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"},
				{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
			},
		},
		{
			User: &User{ID: "user2", UserName: "user_two"},
			Assignments: []*Assigment{
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
		require.NoError(t, c.WaitForAccountAssignmentResult(ctx, deleteResp.RequestID))

		deleteResp, err = c.DeleteAccountAssignment(ctx, &DeleteAccountAssignmentRequest{
			PrincipalID:      "user2",
			PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly",
			PrincipalType:    ssoadmintypes.PrincipalTypeUser,
			AccountID:        "1111111111",
		})
		require.NoError(t, err)
		require.NoError(t, c.WaitForAccountAssignmentResult(ctx, deleteResp.RequestID))

		want := []*UserWithAssignment{
			{
				User: &User{ID: "user1", UserName: "user_one"},
				Assignments: []*Assigment{
					{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
				},
			},
			{
				User:        &User{ID: "user2", UserName: "user_two"},
				Assignments: []*Assigment{},
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
		require.NoError(t, c.WaitForAccountAssignmentResult(ctx, createResp.RequestID))

		want := []*UserWithAssignment{
			{
				User: &User{ID: "user1", UserName: "user_one"},
				Assignments: []*Assigment{
					{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
				},
			},
			{
				User: &User{ID: "user2", UserName: "user_two"},
				Assignments: []*Assigment{
					{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
				},
			},
		}
		assertUsersAssignments(t, c, want)
	})
}

func assertUsersAssignments(t *testing.T, c Client, want []*UserWithAssignment) {
	listResp, err := c.ListUsersWithAccountAndPermAssignment(context.Background())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(listResp, want))

}
