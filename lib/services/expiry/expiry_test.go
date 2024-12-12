/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package expiry

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/utils"
)

func TestExpiry(t *testing.T) {
	clock := clockwork.NewFakeClock()

	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clock,
		AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
			SecondFactor: constants.SecondFactorOn,
			Webauthn: &types.Webauthn{
				RPID: "localhost",
			},
		},
	})

	require.NoError(t, err)
	t.Cleanup(func() { authServer.Close() })

	logger := utils.NewSlogLoggerForTests()
	mockEmitter := &eventstest.MockRecorderEmitter{}
	cfg := &Config{
		Log:         logger,
		Emitter:     mockEmitter,
		AccessPoint: authServer.AuthServer,
		Clock:       clock,
	}

	ctx := context.Background()

	expiry, err := New(cfg)
	require.NoError(t, err)

	scanInterval = time.Second
	pendingRequestGracePeriod = time.Second

	go func() {
		err := expiry.Run(ctx)
		require.NoError(t, err)
	}()

	req1Name := uuid.New().String()
	req1, err := types.NewAccessRequest(req1Name, "someUser", "someRole")
	require.NoError(t, err)
	req1.SetExpiry(clock.Now().Add(scanInterval))
	err = authServer.AuthServer.CreateAccessRequest(ctx, req1)
	require.NoError(t, err)

	req2Name := uuid.New().String()
	req2, err := types.NewAccessRequest(req2Name, "someUser", "someRole")
	require.NoError(t, err)
	req2.SetExpiry(clock.Now().Add(scanInterval * 2))
	err = authServer.AuthServer.CreateAccessRequest(ctx, req2)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		clock.Advance(scanInterval * 5)
		return len(mockEmitter.Events()) == 2
	}, scanInterval*5, time.Second/2)
}
