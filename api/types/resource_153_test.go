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

package types_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestLegacyToResource153(t *testing.T) {
	// user is an example of a legacy resource.
	// Any other resource type would to.
	user := &types.UserV2{
		Kind: "user",
		Metadata: types.Metadata{
			Name: "llama",
		},
		Spec: types.UserSpecV2{
			Roles: []string{"human", "camelidae"},
		},
	}

	resource := types.LegacyToResource153(user)

	// Unwrap gives the underlying resource back.
	t.Run("unwrap", func(t *testing.T) {
		unwrapped := resource.(interface{ Unwrap() types.Resource }).Unwrap()
		if diff := cmp.Diff(user, unwrapped, protocmp.Transform()); diff != "" {
			t.Errorf("Unwrap mismatch (-want +got)\n%s", diff)
		}
	})

	// Marshaling as JSON marshals the underlying resource.
	t.Run("marshal", func(t *testing.T) {
		jsonBytes, err := json.Marshal(resource)
		require.NoError(t, err, "Marshal")

		user2 := &types.UserV2{}
		require.NoError(t, json.Unmarshal(jsonBytes, user2), "Unmarshal")
		if diff := cmp.Diff(user, user2, protocmp.Transform()); diff != "" {
			t.Errorf("Marshal/Unmarshal mismatch (-want +got)\n%s", diff)
		}
	})
}

func TestResource153ToLegacy(t *testing.T) {
	// bot is an example of an RFD 153 "compliant" resource.
	// Any other resource type would do.
	bot := &machineidv1.Bot{
		Kind:     "bot",
		SubKind:  "robot",
		Metadata: &headerv1.Metadata{Name: "Bernard"},
		Spec: &machineidv1.BotSpec{
			Roles: []string{"robot", "human"},
		},
	}

	legacyResource := types.Resource153ToLegacy(bot)

	// Unwrap gives the underlying resource back.
	t.Run("unwrap", func(t *testing.T) {
		unwrapped := legacyResource.(interface{ Unwrap() types.Resource153 }).Unwrap()
		if diff := cmp.Diff(bot, unwrapped, protocmp.Transform()); diff != "" {
			t.Errorf("Unwrap mismatch (-want +got)\n%s", diff)
		}
	})

	// Marshaling as JSON marshals the underlying resource.
	t.Run("marshal", func(t *testing.T) {
		jsonBytes, err := json.Marshal(legacyResource)
		require.NoError(t, err, "Marshal")

		bot2 := &machineidv1.Bot{}
		require.NoError(t, json.Unmarshal(jsonBytes, bot2), "Unmarshal")
		if diff := cmp.Diff(bot, bot2, protocmp.Transform()); diff != "" {
			t.Errorf("Marshal/Unmarshal mismatch (-want +got)\n%s", diff)
		}
	})
}

func TestResourceMethods(t *testing.T) {
	clock := clockwork.NewFakeClock()
	expiry := clock.Now().UTC()

	// user is an example of a legacy resource.
	// Any other resource type would to.
	user := &types.UserV2{
		Kind: "user",
		Metadata: types.Metadata{
			Name:     "llama",
			Expires:  &expiry,
			ID:       1234,
			Revision: "alpaca",
			Labels: map[string]string{
				types.OriginLabel: "earth",
			},
		},
		Spec: types.UserSpecV2{
			Roles: []string{"human", "camelidae"},
		},
	}

	// bot is an example of an RFD 153 "compliant" resource.
	// Any other resource type would do.
	bot := &machineidv1.Bot{
		Kind:    "bot",
		SubKind: "robot",
		Metadata: &headerv1.Metadata{
			Name:     "Bernard",
			Expires:  timestamppb.New(expiry),
			Id:       4567,
			Revision: "tinman",
			Labels: map[string]string{
				types.OriginLabel: "mars",
			},
		},
		Spec: &machineidv1.BotSpec{
			Roles: []string{"robot", "human"},
		},
	}

	t.Run("GetExpiry", func(t *testing.T) {
		_, err := types.GetExpiry("invalid type")
		require.Error(t, err)

		objExpiry, err := types.GetExpiry(user)
		require.NoError(t, err)
		require.Equal(t, expiry, objExpiry)

		objExpiry, err = types.GetExpiry(bot)
		require.NoError(t, err)
		require.Equal(t, expiry, objExpiry)

		// check the nil expiry special case.
		user.Metadata.Expires = nil
		objExpiry, err = types.GetExpiry(user)
		require.NoError(t, err)
		require.Equal(t, time.Time{}, objExpiry)

		bot.Metadata.Expires = nil
		objExpiry, err = types.GetExpiry(bot)
		require.NoError(t, err)
		require.Equal(t, time.Time{}, objExpiry)
	})

	t.Run("GetResourceID", func(t *testing.T) {
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		_, err := types.GetResourceID("invalid type")
		require.Error(t, err)

		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		id, err := types.GetResourceID(user)
		require.NoError(t, err)
		require.Equal(t, user.GetResourceID(), id)

		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		id, err = types.GetResourceID(bot)
		require.NoError(t, err)
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		require.Equal(t, bot.GetMetadata().Id, id)
	})

	t.Run("GetRevision", func(t *testing.T) {
		_, err := types.GetRevision("invalid type")
		require.Error(t, err)

		revision, err := types.GetRevision(user)
		require.Equal(t, user.GetRevision(), revision)
		require.NoError(t, err)

		revision, err = types.GetRevision(bot)
		require.NoError(t, err)
		require.Equal(t, bot.GetMetadata().Revision, revision)
	})

	t.Run("SetRevision", func(t *testing.T) {
		rev := uuid.NewString()
		require.NoError(t, types.SetRevision(bot, rev))
		require.NoError(t, types.SetRevision(user, rev))
		require.Error(t, types.SetRevision("invalid type", "dummy"))

		revision, err := types.GetRevision(user)
		require.NoError(t, err)
		require.Equal(t, rev, revision)

		revision, err = types.GetRevision(bot)
		require.NoError(t, err)
		require.Equal(t, rev, revision)
	})

	t.Run("GetKind", func(t *testing.T) {
		_, err := types.GetKind("invalid type")
		require.Error(t, err)

		kind, err := types.GetKind(user)
		require.NoError(t, err)
		require.Equal(t, types.KindUser, kind)

		kind, err = types.GetKind(bot)
		require.NoError(t, err)
		require.Equal(t, types.KindBot, kind)
	})

	t.Run("GetOrigin", func(t *testing.T) {
		_, err := types.GetOrigin("invalid type")
		require.Error(t, err)

		origin, err := types.GetOrigin(user)
		require.NoError(t, err)
		require.Equal(t, user.Origin(), origin)

		origin, err = types.GetOrigin(bot)
		require.NoError(t, err)
		require.Equal(t, "mars", origin)
	})
}

// Tests that expiry is consistent across the different types and transformations.
func TestExpiryConsistency(t *testing.T) {
	tests := []struct {
		name            string
		expiryTimestamp *timestamppb.Timestamp
		expectedExpiry  time.Time
	}{
		{
			name:            "nil expiry",
			expiryTimestamp: nil,
			expectedExpiry:  time.Time{},
		},
		{
			name:            "zero expiry",
			expiryTimestamp: timestamppb.New(time.Time{}),
			expectedExpiry:  time.Time{},
		},
		{
			name:            "set expiry",
			expiryTimestamp: timestamppb.New(time.Date(2024, 11, 11, 11, 11, 11, 00, time.UTC)),
			expectedExpiry:  time.Date(2024, 11, 11, 11, 11, 11, 00, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bot := &machineidv1.Bot{
				Kind:     "bot",
				SubKind:  "robot",
				Metadata: &headerv1.Metadata{Name: "Bernard", Expires: tt.expiryTimestamp},
				Spec: &machineidv1.BotSpec{
					Roles: []string{"robot", "human"},
				},
			}

			legacyResource := types.Resource153ToLegacy(bot)

			// verify expiry time in different ways
			t.Run("GetExpiry() resource", func(t *testing.T) {
				expiry, err := types.GetExpiry(bot)
				require.NoError(t, err)
				require.Equal(t, tt.expectedExpiry, expiry)
			})

			t.Run("GetExpiry() wrapper", func(t *testing.T) {
				expiry, err := types.GetExpiry(legacyResource)
				require.NoError(t, err)
				require.Equal(t, tt.expectedExpiry, expiry)
			})

			t.Run("wrapper .Expiry()", func(t *testing.T) {
				require.Equal(t, tt.expectedExpiry, legacyResource.Expiry())
			})

			t.Run("wrapper metadata .Expiry()", func(t *testing.T) {
				md := legacyResource.GetMetadata()
				require.Equal(t, tt.expectedExpiry, md.Expiry())
			})
		})
	}
}
