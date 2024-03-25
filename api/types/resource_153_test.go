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
	// fake153Resource is used here because, at the time of backport, there were
	// no RFD 153 resources in this branch.
	bot := &fake153Resource{
		Kind:     "bot",
		SubKind:  "robot",
		Metadata: &headerv1.Metadata{Name: "Bernard"},
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

		bot2 := &fake153Resource{}
		require.NoError(t, json.Unmarshal(jsonBytes, bot2), "Unmarshal")
		if diff := cmp.Diff(bot, bot2, protocmp.Transform()); diff != "" {
			t.Errorf("Marshal/Unmarshal mismatch (-want +got)\n%s", diff)
		}
	})
}

type fake153Resource struct {
	Kind     string
	SubKind  string
	Version  string
	Metadata *headerv1.Metadata
}

func (r *fake153Resource) GetKind() string {
	return r.Kind
}

func (r *fake153Resource) GetMetadata() *headerv1.Metadata {
	return r.Metadata
}

func (r *fake153Resource) GetSubKind() string {
	return r.SubKind
}

func (r *fake153Resource) GetVersion() string {
	return r.Version
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
	bot := &fake153Resource{
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
	}

	t.Run("GetExpiry", func(t *testing.T) {
		require.Equal(t, expiry, types.GetExpiry(user))
		require.Equal(t, expiry, types.GetExpiry(bot))

		// check the nil expiry special case.
		user.Metadata.Expires = nil
		require.Equal(t, time.Time{}, types.GetExpiry(user))

		bot.Metadata.Expires = nil
		require.Equal(t, time.Time{}, types.GetExpiry(bot))
	})

	t.Run("GetResourceID", func(t *testing.T) {
		//nolint:staticcheck // SA1019. Added for backward compatibility.
		require.Equal(t, user.GetResourceID(), types.GetResourceID(user))
		//nolint:staticcheck // SA1019. Added for backward compatibility.
		require.Equal(t, bot.GetMetadata().Id, types.GetResourceID(bot))
	})

	t.Run("GetRevision", func(t *testing.T) {
		require.Equal(t, user.GetRevision(), types.GetRevision(user))
		require.Equal(t, bot.GetMetadata().Revision, types.GetRevision(bot))
	})

	t.Run("SetRevision", func(t *testing.T) {
		rev := uuid.NewString()
		types.SetRevision(bot, rev)
		types.SetRevision(user, rev)

		require.Equal(t, rev, types.GetRevision(user))
		require.Equal(t, rev, types.GetRevision(bot))
	})

	t.Run("GetKind", func(t *testing.T) {
		require.Equal(t, types.KindUser, types.GetKind(user))
		require.Equal(t, "bot", types.GetKind(bot))
	})

	t.Run("GetOrigin", func(t *testing.T) {
		require.Equal(t, user.Origin(), types.GetOrigin(user))
		require.Equal(t, "mars", types.GetOrigin(bot))
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
