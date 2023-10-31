// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/predicate/builder"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestUserCRUDRBAC(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create a read-only user
	roAllowRules := []types.Rule{
		types.NewRule(types.KindUser, services.RO()),
	}
	roUser, _, err := CreateUserAndRole(srv.Auth(), "ro-user", nil, roAllowRules)
	require.NoError(t, err)

	// Create a global-rw user
	rwAllowRules := []types.Rule{
		types.NewRule(types.KindUser, services.RO()),
		types.NewRule(types.KindUser, services.RW()),
	}
	rwUser, _, err := CreateUserAndRole(srv.Auth(), "rw-user", nil, rwAllowRules)
	require.NoError(t, err)

	// Create user with conditional read-write access
	targetedAllowRules := []types.Rule{
		types.NewRule(types.KindUser, services.RO()),
		{
			Resources: []string{types.KindUser},
			Verbs:     []string{types.VerbCreate, types.VerbUpdate, types.VerbDelete},
			Where: builder.Equals(
				builder.Identifier(`resource.metadata.labels["foo"]`),
				builder.String("bar"),
			).String(),
		},
	}
	targetedAllowUser, _, err := CreateUserAndRole(srv.Auth(), "targeted-allow-user", nil, targetedAllowRules)
	require.NoError(t, err)

	// Create a user where a general allow-RW rule is overridden by a
	// label-matched Deny rule
	targetedDenyUser, targetedDenyRole, err := CreateUserAndRole(srv.Auth(), "targeted-deny-user", nil, rwAllowRules)
	require.NoError(t, err)
	targetedDenyRole.SetRules(types.Deny, []types.Rule{
		{
			Resources: []string{types.KindUser},
			Verbs:     []string{types.VerbCreate, types.VerbUpdate, types.VerbDelete},
			Where: builder.Equals(
				builder.Identifier(`resource.metadata.labels["foo"]`),
				builder.String("bar"),
			).String(),
		},
	})
	_, err = srv.Auth().UpsertRole(ctx, targetedDenyRole)
	require.NoError(t, err)

	// Creates a Teleport client that acts as the supplied user
	testClientForUser := func(innerT *testing.T, user types.User) *Client {
		client, err := srv.NewClient(TestUser(user.GetName()))
		require.NoError(innerT, err)
		innerT.Cleanup(func() { require.NoError(innerT, client.Close()) })
		return client
	}

	// Creates a Teleport user (including inserting it into the cluster user
	// DB), or fail the test
	makeTestUser := func(innerT *testing.T, labels map[string]string) types.User {
		u, err := types.NewUser(innerT.Name())
		require.NoError(innerT, err)

		if len(labels) != 0 {
			meta := u.GetMetadata()
			meta.Labels = labels
			u.SetMetadata(meta)
		}

		result, err := srv.Auth().CreateUser(ctx, u)
		require.NoError(innerT, err)

		return result
	}

	// All of the various non-matching label test cases for asserting that a
	// non-matching label set won't allow writing
	labelTestCases := []struct {
		name   string
		labels map[string]string
	}{
		{"empty label set", map[string]string{}},
		{"label key mismatch", map[string]string{"nope": "bar"}},
		{"label value mismatch", map[string]string{"foo": "nope"}},
	}

	t.Run("Create", func(t *testing.T) {

		t.Run("denied-without-allow-rule", func(t *testing.T) {
			roClient := testClientForUser(t, roUser)

			user, err := types.NewUser(t.Name())
			require.NoError(t, err)

			_, err = roClient.CreateUser(ctx, user)
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		})

		t.Run("allowed-with-allow-rule", func(t *testing.T) {
			rwClient := testClientForUser(t, rwUser)

			user, err := types.NewUser(t.Name())
			require.NoError(t, err)

			newUser, err := rwClient.CreateUser(ctx, user)
			require.NoError(t, err)
			require.Equal(t, user.GetName(), newUser.GetName())
		})

		t.Run("denied-with-unmatched-targeted-allow-rule", func(t *testing.T) {
			targetedAllowClient := testClientForUser(t, targetedAllowUser)

			for _, test := range labelTestCases {
				t.Run(test.name, func(t *testing.T) {
					user, err := types.NewUser(t.Name())
					require.NoError(t, err)

					m := user.GetMetadata()
					m.Labels = test.labels
					user.SetMetadata(m)

					_, err = targetedAllowClient.CreateUser(ctx, user)
					require.Error(t, err)
					require.True(t, trace.IsAccessDenied(err))
				})
			}
		})

		t.Run("allowed-with-matching-where-clause", func(t *testing.T) {
			targetAllowClient := testClientForUser(t, targetedAllowUser)

			user, err := types.NewUser(t.Name())
			require.NoError(t, err)
			m := user.GetMetadata()
			m.Labels = map[string]string{"foo": "bar"}
			user.SetMetadata(m)

			_, err = targetAllowClient.CreateUser(ctx, user)
			require.NoError(t, err)
		})

		t.Run("denied-with-targeted-deny-rule", func(t *testing.T) {
			targetDenyClient := testClientForUser(t, targetedDenyUser)

			user, err := types.NewUser(t.Name())
			require.NoError(t, err)
			m := user.GetMetadata()
			m.Labels = map[string]string{"foo": "bar"}
			user.SetMetadata(m)

			_, err = targetDenyClient.CreateUser(ctx, user)
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("denied-without-allow-rule", func(t *testing.T) {
			roClient := testClientForUser(t, roUser)

			targetUser := makeTestUser(t, nil)
			targetUser.AddRole("access")

			_, err = roClient.UpdateUser(ctx, targetUser)
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		})

		t.Run("allowed-with-allow-rule", func(t *testing.T) {
			rwClient := testClientForUser(t, rwUser)

			targetUser := makeTestUser(t, nil)
			targetUser.SetTraits(map[string][]string{
				"favorite_fruit": {"banana"},
			})

			_, err := rwClient.UpdateUser(ctx, targetUser)
			require.NoError(t, err)
		})

		t.Run("denied-with-unmatched-targeted-allow-rule", func(t *testing.T) {
			targetedAllowClient := testClientForUser(t, targetedAllowUser)

			for _, testCase := range labelTestCases {
				t.Run(testCase.name, func(t *testing.T) {
					targetUser := makeTestUser(t, testCase.labels)

					targetUser.SetTraits(map[string][]string{
						"favorite_fruit": {"banana"},
					})
					_, err := targetedAllowClient.UpdateUser(ctx, targetUser)
					require.Error(t, err)
					require.True(t, trace.IsAccessDenied(err))
				})
			}
		})

		t.Run("allowed-with-targeted-allow-rule", func(t *testing.T) {
			targetUser := makeTestUser(t, map[string]string{"foo": "bar"})
			targetUser.SetTraits(map[string][]string{
				"favorite_fruit": {"banana"},
			})
			targetedAllowClient := testClientForUser(t, targetedAllowUser)
			_, err := targetedAllowClient.UpdateUser(ctx, targetUser)
			require.NoError(t, err)
		})

		t.Run("denied-with-targeted-deny-rule", func(t *testing.T) {
			targetUser := makeTestUser(t, map[string]string{"foo": "bar"})
			targetUser.SetTraits(map[string][]string{
				"favorite_fruit": {"banana"},
			})
			targetedDenyClient := testClientForUser(t, targetedDenyUser)
			_, err := targetedDenyClient.UpdateUser(ctx, targetUser)
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("Upsert", func(t *testing.T) {
		t.Run("creation-denied-without-allow-rule", func(t *testing.T) {
			roClient := testClientForUser(t, roUser)

			user, err := types.NewUser(t.Name())
			require.NoError(t, err)

			_, err = roClient.UpsertUser(ctx, user)
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		})

		t.Run("denied-without-create-right", func(t *testing.T) {
			// Given a user with update rights (but not creator rights...
			updateOnlyRules := []types.Rule{
				types.NewRule(types.KindUser, services.RO()),
				types.NewRule(types.KindUser, []string{types.VerbUpdate}),
			}
			updateOnlyUser, _, err := CreateUserAndRole(srv.Auth(), "update-only-user", nil, updateOnlyRules)
			require.NoError(t, err)
			updateOnlyClient := testClientForUser(t, updateOnlyUser)

			// When I try to create a new user via upsert, expect that it fails
			// with AccessDenied
			targetUser, err := types.NewUser(t.Name())
			require.NoError(t, err)

			_, err = updateOnlyClient.UpsertUser(ctx, targetUser)
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))

			// When I try to update an existing user, expect that it fails with
			// AccessDenied, even though the test user nominally has the create
			// right.
			targetUser = makeTestUser(t, nil)
			_, err = updateOnlyClient.UpsertUser(ctx, targetUser)
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))

			// When I try to update an existing user via an upsert from a client
			// with full RW a full RW privileges, it succeeds
			rwClient := testClientForUser(t, rwUser)
			_, err = rwClient.UpsertUser(ctx, targetUser)
			require.NoError(t, err)
		})

		t.Run("update-denied-without-allow-rule", func(t *testing.T) {
			rwClient := testClientForUser(t, rwUser)

			targetUser := makeTestUser(t, nil)
			targetUser.SetTraits(map[string][]string{
				"favorite_fruit": {"banana"},
			})

			_, err := rwClient.UpsertUser(ctx, targetUser)
			require.NoError(t, err)
		})

		t.Run("denied-with-unmatched-targeted-allow-rule", func(t *testing.T) {
			targetedAllowClient := testClientForUser(t, targetedAllowUser)

			for _, testCase := range labelTestCases {
				t.Run(testCase.name, func(t *testing.T) {
					user, err := types.NewUser(t.Name())
					require.NoError(t, err)

					m := user.GetMetadata()
					m.Labels = testCase.labels
					user.SetMetadata(m)

					// When I try to create new user via upsert, expect that it
					// fails
					_, err = targetedAllowClient.UpsertUser(ctx, user)
					require.Error(t, err)
					require.True(t, trace.IsAccessDenied(err))

					// when I try to update existing user via upsert, expect
					// that it fails
					_, err = srv.Auth().CreateUser(ctx, user)
					require.NoError(t, err)

					_, err = targetedAllowClient.UpsertUser(ctx, user)
					require.Error(t, err)
					require.True(t, trace.IsAccessDenied(err))
				})
			}
		})

		t.Run("allowed-with-targeted-allow-rule", func(t *testing.T) {
			targetUser := makeTestUser(t, map[string]string{"foo": "bar"})
			targetUser.SetTraits(map[string][]string{
				"favorite_fruit": {"banana"},
			})

			// When I try to create new user via upsert, expect that it succeeds
			targetedAllowClient := testClientForUser(t, targetedAllowUser)
			targetUser, err = targetedAllowClient.UpsertUser(ctx, targetUser)
			require.NoError(t, err)

			// When I try to update an existing user (crated via previous test
			// step) via upsert, expect that it succeeds
			targetUser.SetLogins([]string{"root"})
			_, err := targetedAllowClient.UpsertUser(ctx, targetUser)
			require.NoError(t, err)
		})

		t.Run("denied-with-targeted-deny-rule", func(t *testing.T) {
			// Given a Teleport client that has a rule to deny write operations
			// on a user with the label "foo": "bar"
			targetedDenyClient := testClientForUser(t, targetedDenyUser)

			// ... and a user with that label
			targetUser, err := types.NewUser(t.Name())
			require.NoError(t, err)
			m := targetUser.GetMetadata()
			m.Labels = map[string]string{"foo": "bar"}
			targetUser.SetMetadata(m)

			// When I try to create a new user labeled with "foo": "bar"
			// via upsert, expect that it fails
			_, err = targetedDenyClient.UpsertUser(ctx, targetUser)
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))

			// Given an existing user of the same name...
			targetUser, err = srv.Auth().CreateUser(ctx, targetUser)
			require.NoError(t, err)

			// When I try to update that user via upsert it fails
			targetUser.SetLogins([]string{"root"})
			_, err = targetedDenyClient.UpsertUser(ctx, targetUser)
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Run("denied-without-allow-rule", func(t *testing.T) {
			targetUser := makeTestUser(t, nil)

			roClient := testClientForUser(t, roUser)
			err = roClient.DeleteUser(ctx, targetUser.GetName())
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		})

		t.Run("allowed-with-allow-rule", func(t *testing.T) {
			targetUser := makeTestUser(t, nil)

			rwClient := testClientForUser(t, rwUser)
			err := rwClient.DeleteUser(ctx, targetUser.GetName())
			require.NoError(t, err)
		})

		t.Run("denied-with-unmatched-targeted-allow-rule", func(t *testing.T) {
			targetedAllowClient := testClientForUser(t, targetedAllowUser)

			for _, testCase := range labelTestCases {
				t.Run(testCase.name, func(t *testing.T) {
					targetUser := makeTestUser(t, testCase.labels)
					err := targetedAllowClient.DeleteUser(ctx, targetUser.GetName())
					require.Error(t, err)
					require.True(t, trace.IsAccessDenied(err))
				})
			}
		})

		t.Run("allowed-with-targeted-allow-rule", func(t *testing.T) {
			targetUser := makeTestUser(t, map[string]string{"foo": "bar"})

			targetedAllowClient := testClientForUser(t, targetedAllowUser)
			err := targetedAllowClient.DeleteUser(ctx, targetUser.GetName())
			require.NoError(t, err)
		})

		t.Run("denied-with-targeted-deny-rule", func(t *testing.T) {
			targetUser := makeTestUser(t, map[string]string{"foo": "bar"})

			targetedDenyClient := testClientForUser(t, targetedDenyUser)
			err := targetedDenyClient.DeleteUser(ctx, targetUser.GetName())
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err))
		})
	})
}
