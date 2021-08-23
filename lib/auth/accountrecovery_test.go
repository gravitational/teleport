/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type testWithCloudModules struct {
	modules.Modules
}

func (m *testWithCloudModules) Features() modules.Features {
	return modules.Features{
		Cloud: true, // Enable cloud feature which is required for account recovery.
	}
}

// TestGenerateAndUpsertRecoveryCodes tests the following:
//  - generation of recovery codes are of correct format
//  - recovery codes are upserted
//  - recovery codes can be verified and marked used
//  - reusing a used or non-existing token returns error
func TestGenerateAndUpsertRecoveryCodes(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	user := "fake@fake.com"
	rc, err := srv.Auth().generateAndUpsertRecoveryCodes(ctx, user)
	require.NoError(t, err)
	require.Len(t, rc, 3)

	// Test each codes are of correct format and used.
	for _, token := range rc {
		s := strings.Split(token, "-")

		// 9 b/c 1 for prefix, 8 for words.
		require.Len(t, s, 9)
		require.Contains(t, token, "tele-")

		// Test codes match.
		err := srv.Auth().verifyRecoveryCode(ctx, user, []byte(token))
		require.NoError(t, err)
	}

	// Test used codes are marked used.
	recovery, err := srv.Auth().GetRecoveryCodes(ctx, user)
	require.NoError(t, err)
	for _, token := range recovery.GetCodes() {
		require.True(t, token.IsUsed)
	}

	// Test with a used code returns error.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte(rc[0]))
	require.True(t, trace.IsBadParameter(err))

	// Test with invalid recoery code returns error.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte("invalidcode"))
	require.True(t, trace.IsBadParameter(err))

	// Test with non-existing user returns error.
	err = srv.Auth().verifyRecoveryCode(ctx, "doesnotexist", []byte(rc[0]))
	require.True(t, trace.IsBadParameter(err))
}

func TestRecoveryCodeEventsEmitted(t *testing.T) {
	srv := newTestTLSServer(t)
	ctx := context.Background()
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	user := "fake@fake.com"

	// Test generated recovery codes event.
	tc, err := srv.Auth().generateAndUpsertRecoveryCodes(ctx, user)
	require.NoError(t, err)
	event := mockEmitter.LastEvent()
	require.Equal(t, events.RecoveryCodeGeneratedEvent, event.GetType())
	require.Equal(t, events.RecoveryCodesGeneratedCode, event.GetCode())

	// Test used recovery code event.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte(tc[0]))
	require.NoError(t, err)
	event = mockEmitter.LastEvent()
	require.Equal(t, events.RecoveryCodeUsedEvent, event.GetType())
	require.Equal(t, events.RecoveryCodeUsedCode, event.GetCode())

	// Re-using the same token emits failed event.
	err = srv.Auth().verifyRecoveryCode(ctx, user, []byte(tc[0]))
	require.Error(t, err)
	event = mockEmitter.LastEvent()
	require.Equal(t, events.RecoveryCodeUsedEvent, event.GetType())
	require.Equal(t, events.RecoveryCodeUsedFailureCode, event.GetCode())
}
