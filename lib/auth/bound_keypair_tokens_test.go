package auth_test

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
)

func TestUpsertBoundKeypairToken(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	clock := clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC())
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{Dir: t.TempDir(), Clock: clock})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, as.Close()) })
	authServer := as.AuthServer

	tests := []struct {
		name string

		pre         *types.ProvisionTokenV2
		token       *types.ProvisionTokenV2
		assertError require.ErrorAssertionFunc
		assertToken func(t *testing.T, token *types.ProvisionTokenV2)
	}{
		{
			name: "minimal create results in valid status",
			token: &types.ProvisionTokenV2{
				Kind:    types.KindToken,
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "minimal",
				},
				Spec: types.ProvisionTokenSpecV2{
					JoinMethod:   types.JoinMethodBoundKeypair,
					Roles:        []types.SystemRole{types.RoleBot},
					BotName:      "test",
					BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{},
				},
			},
			assertToken: func(t *testing.T, token *types.ProvisionTokenV2) {
				// Should generate a registration secret but nothing else
				require.NotEmpty(t, token.Status.BoundKeypair.RegistrationSecret)
				require.Empty(t, token.Status.BoundKeypair.BoundBotInstanceID)
				require.Empty(t, token.Status.BoundKeypair.BoundHostID)
				require.Empty(t, token.Status.BoundKeypair.BoundPublicKey)
			},
		},
		{
			name: "create with predefined registration secret copies the value",
			token: &types.ProvisionTokenV2{
				Kind:    types.KindToken,
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "predefined-secret",
				},
				Spec: types.ProvisionTokenSpecV2{
					JoinMethod: types.JoinMethodBoundKeypair,
					Roles:      []types.SystemRole{types.RoleBot},
					BotName:    "test",
					BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{
						Onboarding: &types.ProvisionTokenSpecV2BoundKeypair_OnboardingSpec{
							RegistrationSecret: "asdf",
						},
					},
				},
			},
			assertToken: func(t *testing.T, token *types.ProvisionTokenV2) {
				require.Equal(t, "asdf", token.Status.BoundKeypair.RegistrationSecret)
			},
		},
		{
			name: "create with predefined public key does NOT copy the value",
			token: &types.ProvisionTokenV2{
				Kind:    types.KindToken,
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "initial-public-key",
				},
				Spec: types.ProvisionTokenSpecV2{
					JoinMethod: types.JoinMethodBoundKeypair,
					Roles:      []types.SystemRole{types.RoleBot},
					BotName:    "test",
					BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{
						Onboarding: &types.ProvisionTokenSpecV2BoundKeypair_OnboardingSpec{
							InitialPublicKey: "asdf",
						},
					},
				},
			},
			assertToken: func(t *testing.T, token *types.ProvisionTokenV2) {
				// Keys only become bound at join-time, not at upsert, so it
				// should remain empty.
				require.Empty(t, token.Status.BoundKeypair.RegistrationSecret)
				require.Empty(t, token.Status.BoundKeypair.BoundPublicKey)
			},
		},
		{
			name: "create with status preserves values",
			token: &types.ProvisionTokenV2{
				Kind:    types.KindToken,
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "create-with-status",
				},
				Spec: types.ProvisionTokenSpecV2{
					JoinMethod:   types.JoinMethodBoundKeypair,
					Roles:        []types.SystemRole{types.RoleBot},
					BotName:      "test",
					BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{},
				},
				Status: &types.ProvisionTokenStatusV2{
					// It's blatantly invalid to set every field, but we just
					// want to make sure upsert is not deleting anything.
					BoundKeypair: &types.ProvisionTokenStatusV2BoundKeypair{
						RegistrationSecret: "registration-secret",
						BoundPublicKey:     "bound-public-key",
						BoundBotInstanceID: "bot-instance-id",
						BoundHostID:        "host-id",
						RecoveryCount:      123,
						LastRecoveredAt:    new(time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)),
						LastRotatedAt:      new(time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)),
					},
				},
			},
			assertToken: func(t *testing.T, token *types.ProvisionTokenV2) {
				// On create, all fields should be copied.
				require.Equal(t, &types.ProvisionTokenStatusV2BoundKeypair{
					RegistrationSecret: "registration-secret",
					BoundPublicKey:     "bound-public-key",
					BoundBotInstanceID: "bot-instance-id",
					BoundHostID:        "host-id",
					RecoveryCount:      123,
					LastRecoveredAt:    new(time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)),
					LastRotatedAt:      new(time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)),
				}, token.Status.BoundKeypair)
			},
		},
		{
			name: "update with status preserves stored values",
			pre: &types.ProvisionTokenV2{
				Kind:    types.KindToken,
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "update-with-status",
				},
				Spec: types.ProvisionTokenSpecV2{
					JoinMethod: types.JoinMethodBoundKeypair,
					Roles:      []types.SystemRole{types.RoleBot},
					BotName:    "test",
					BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{
						Recovery: &types.ProvisionTokenSpecV2BoundKeypair_RecoverySpec{
							Limit: 2,
							Mode:  "standard",
						},
					},
				},
				Status: &types.ProvisionTokenStatusV2{
					// As before, setting every field is invalid (at runtime),
					// but we want to be sure nothing is overwritten.
					BoundKeypair: &types.ProvisionTokenStatusV2BoundKeypair{
						RegistrationSecret: "registration-secret",
						BoundPublicKey:     "bound-public-key",
						BoundBotInstanceID: "bot-instance-id",
						BoundHostID:        "host-id",
						RecoveryCount:      123,
						LastRecoveredAt:    new(time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)),
						LastRotatedAt:      new(time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)),
					},
				},
			},
			token: &types.ProvisionTokenV2{
				Kind:    types.KindToken,
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "update-with-status",
				},
				Spec: types.ProvisionTokenSpecV2{
					JoinMethod: types.JoinMethodBoundKeypair,
					Roles:      []types.SystemRole{types.RoleBot},
					BotName:    "test",
					BoundKeypair: &types.ProvisionTokenSpecV2BoundKeypair{
						Recovery: &types.ProvisionTokenSpecV2BoundKeypair_RecoverySpec{
							Limit: 5,
							Mode:  "relaxed",
						},
					},
				},
				Status: &types.ProvisionTokenStatusV2{
					// Set every field to some other value.
					BoundKeypair: &types.ProvisionTokenStatusV2BoundKeypair{
						RegistrationSecret: "other-registration-secret",
						BoundPublicKey:     "other-bound-public-key",
						BoundBotInstanceID: "other-bot-instance-id",
						BoundHostID:        "other-host-id",
						RecoveryCount:      456,
						LastRecoveredAt:    new(time.Date(2025, 2, 1, 1, 0, 0, 0, time.UTC)),
						LastRotatedAt:      new(time.Date(2025, 2, 1, 1, 0, 0, 0, time.UTC)),
					},
				},
			},
			assertToken: func(t *testing.T, token *types.ProvisionTokenV2) {
				// All status values should equal the _original_ token, not the
				// updated one
				require.Equal(t, &types.ProvisionTokenStatusV2BoundKeypair{
					RegistrationSecret: "registration-secret",
					BoundPublicKey:     "bound-public-key",
					BoundBotInstanceID: "bot-instance-id",
					BoundHostID:        "host-id",
					RecoveryCount:      123,
					LastRecoveredAt:    new(time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)),
					LastRotatedAt:      new(time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)),
				}, token.Status.BoundKeypair)

				// Spec field updates should be honored.
				require.EqualValues(t, 5, token.Spec.BoundKeypair.Recovery.Limit)
				require.Equal(t, "relaxed", token.Spec.BoundKeypair.Recovery.Mode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pre != nil {
				require.NoError(t, authServer.UpsertBoundKeypairToken(ctx, tt.pre))
			}

			err := authServer.UpsertBoundKeypairToken(ctx, tt.token)
			if tt.assertError != nil {
				tt.assertError(t, err)
			} else {
				require.NoError(t, err)
			}

			if err != nil {
				return
			}

			token, err := authServer.GetToken(ctx, tt.token.GetName())
			require.NoError(t, err)

			ptv2, ok := token.(*types.ProvisionTokenV2)
			require.True(t, ok)

			tt.assertToken(t, ptv2)
		})
	}
}
