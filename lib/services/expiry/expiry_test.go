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
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/utils/interval"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestAccessListExpiryBasic(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		expiry, authServer, emitter := setupExpiryService(t)
		go func() {
			err := expiry.Run(ctx)
			require.NoError(t, err)
		}()

		const expiry1, expiry2 = 10 * scanInterval, 20 * scanInterval
		_ = createAccessRequest(t, authServer, types.RequestState_NONE, time.Now().Add(expiry1))
		_ = createAccessRequest(t, authServer, types.RequestState_PROMOTED, time.Now().Add(expiry2))

		synctest.Wait()
		require.Len(t, mustListAccessRequests(t, authServer), 2)
		require.Empty(t, emitter.Events())

		sleep1 := expiry1 + scanInterval*3 // *2 to accommodate for the jitter and initial duration
		time.Sleep(sleep1)
		synctest.Wait()
		require.Len(t, mustListAccessRequests(t, authServer), 1)
		require.Len(t, emitter.Events(), 1)

		sleep2 := expiry2 + scanInterval*3 - sleep1
		time.Sleep(sleep2)
		synctest.Wait()
		require.Empty(t, mustListAccessRequests(t, authServer))
		require.Len(t, emitter.Events(), 2)
	})
}

func TestAccessListExpiryInterval(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		const testInterval = time.Hour

		expiry, authServer, emitter := setupExpiryService(t)
		expiry.expiryTasks[0].intervalCfg = interval.Config{
			Duration:      testInterval,
			FirstDuration: testInterval,
		}
		go func() {
			// Run with rigid intervals.
			err := expiry.run(ctx, expiry.expiryTasks[0])
			require.NoError(t, err)
		}()

		// Create a request with minimal expiry after each interval.
		_ = createAccessRequest(t, authServer, types.RequestState_DENIED, time.Now().Add(1))
		_ = createAccessRequest(t, authServer, types.RequestState_DENIED, time.Now().Add(1+testInterval))
		_ = createAccessRequest(t, authServer, types.RequestState_DENIED, time.Now().Add(1+2*testInterval))

		// Stop just before the first sweep.
		time.Sleep(testInterval - time.Nanosecond)
		synctest.Wait()

		require.Len(t, mustListAccessRequests(t, authServer), 3)
		require.Empty(t, emitter.Events())

		// First sweep.
		time.Sleep(time.Nanosecond)
		synctest.Wait()

		require.Len(t, mustListAccessRequests(t, authServer), 2)
		require.Len(t, emitter.Events(), 1)

		// Stop just before the second sweep.
		time.Sleep(testInterval - time.Nanosecond)
		synctest.Wait()

		require.Len(t, mustListAccessRequests(t, authServer), 2)
		require.Len(t, emitter.Events(), 1)

		// Second sweep.
		time.Sleep(time.Nanosecond)
		synctest.Wait()

		require.Len(t, mustListAccessRequests(t, authServer), 1)
		require.Len(t, emitter.Events(), 2)

		// Stop just before the third sweep.
		time.Sleep(testInterval - time.Nanosecond)
		synctest.Wait()

		require.Len(t, mustListAccessRequests(t, authServer), 1)
		require.Len(t, emitter.Events(), 2)

		// Third sweep.
		time.Sleep(time.Nanosecond)
		synctest.Wait()

		require.Empty(t, mustListAccessRequests(t, authServer))
		require.Len(t, emitter.Events(), 3)
	})
}

func TestAccessListExpiryPendingGracePeriod(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		const testInterval = time.Hour

		expiryService, authServer, emitter := setupExpiryService(t)
		expiryService.expiryTasks[0].intervalCfg = interval.Config{
			Duration:      testInterval,
			FirstDuration: testInterval,
		}
		go func() {
			// Run with rigid intervals.
			err := expiryService.run(ctx, expiryService.expiryTasks[0])
			require.NoError(t, err)
		}()

		expiryTime := time.Now().Add(testInterval - pendingRequestGracePeriod + time.Nanosecond)

		pendingRequest := createAccessRequest(t, authServer, types.RequestState_PENDING, expiryTime)
		approvedRequest := createAccessRequest(t, authServer, types.RequestState_APPROVED, expiryTime)

		// Check both are in the backend.
		require.Len(t, mustListAccessRequests(t, authServer), 2)
		require.Empty(t, emitter.Events())

		// Wait for the expiry service sweep.
		time.Sleep(testInterval)
		synctest.Wait()

		// Make sure both are expired.
		require.True(t, time.Now().After(pendingRequest.Expiry()))
		require.True(t, time.Now().After(approvedRequest.Expiry()))

		// Make sure both are expired within the pending request grace period.
		require.Less(t, time.Since(pendingRequest.Expiry()), pendingRequestGracePeriod)
		require.Less(t, time.Since(approvedRequest.Expiry()), pendingRequestGracePeriod)

		// Check the approved one expired, but the pending one is still within the grace period.
		require.Len(t, mustListAccessRequests(t, authServer), 1)
		require.Equal(t, types.RequestState_PENDING, mustListAccessRequests(t, authServer)[0].GetState())
		require.Len(t, emitter.Events(), 1)

		// Wait for the second expiry service sweep.
		time.Sleep(testInterval)
		synctest.Wait()

		// We are after the grace period so check everything is cleared now.
		require.Greater(t, testInterval, pendingRequestGracePeriod)
		require.Empty(t, mustListAccessRequests(t, authServer))
		require.Len(t, emitter.Events(), 2)
	})
}

func TestAppSessionExpiry(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		expiry, authServer, emitter := setupExpiryService(t)
		go func() {
			err := expiry.Run(ctx)
			require.NoError(t, err)
		}()

		const expiry1 = 5 * scanInterval
		const expiry2 = 10 * scanInterval

		createAppSession(t, authServer, "alice", time.Now().Add(expiry1))
		createAppSession(t, authServer, "bob", time.Now().Add(expiry2))

		synctest.Wait()
		require.Len(t, mustListAppSessions(t, authServer), 2)
		require.Empty(t, emitter.Events())

		time.Sleep(expiry1 + scanInterval*2)
		synctest.Wait()

		sessions := mustListAppSessions(t, authServer)
		require.Len(t, sessions, 1)
		require.Equal(t, "bob", sessions[0].GetUser())

		require.Len(t, emitter.Events(), 1)

		time.Sleep(expiry2)
		synctest.Wait()

		require.Empty(t, mustListAppSessions(t, authServer))
		require.Len(t, emitter.Events(), 2)
	})
}

func TestAppSessionExpiryInvalidTLSCert(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		const testInterval = time.Hour

		expiry, authServer, emitter := setupExpiryService(t)
		expiry.expiryTasks[1].intervalCfg = interval.Config{
			Duration:      testInterval,
			FirstDuration: testInterval,
		}
		go func() {
			err := expiry.run(ctx, expiry.expiryTasks[1])
			require.NoError(t, err)
		}()

		// Create a session and update it with an invalid TLS cert
		session := createAppSession(t, authServer, "alice", time.Now().Add(1))
		session, err := authServer.GetAppSession(ctx, types.GetAppSessionRequest{SessionID: session.GetName()})
		require.NoError(t, err)
		sessionV2, ok := session.(*types.WebSessionV2)
		require.True(t, ok)
		sessionV2.Spec.TLSCert = []byte("not-a-cert")
		err = authServer.UpdateAppSession(ctx, session)
		require.NoError(t, err)

		time.Sleep(testInterval)
		synctest.Wait()

		// Session should still be expired and emitted, but without some app details
		require.Empty(t, mustListAppSessions(t, authServer))
		require.Len(t, emitter.Events(), 1)
		expireEvent, ok := emitter.Events()[0].(*apievents.AppSessionExpire)
		require.True(t, ok)
		require.Equal(t, session.GetName(), expireEvent.SessionID)
		require.Equal(t, "alice", expireEvent.User)
		require.Empty(t, expireEvent.AppPublicAddr)
		require.Empty(t, expireEvent.AppName)
		require.Zero(t, expireEvent.AppTargetPort)
	})
}

func TestAppSessionExpiryInterval(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		const testInterval = time.Hour

		expiry, authServer, emitter := setupExpiryService(t)
		expiry.expiryTasks[1].intervalCfg = interval.Config{
			Duration:      testInterval,
			FirstDuration: testInterval,
		}
		go func() {
			// Run with rigid intervals.
			err := expiry.run(ctx, expiry.expiryTasks[1])
			require.NoError(t, err)
		}()

		createAppSession(t, authServer, "alice", time.Now().Add(1))
		createAppSession(t, authServer, "bob", time.Now().Add(testInterval+time.Nanosecond))
		createAppSession(t, authServer, "charlie", time.Now().Add(2*testInterval+time.Nanosecond))

		// Stop just before the first sweep.
		time.Sleep(testInterval - time.Nanosecond)
		synctest.Wait()
		sessions := mustListAppSessions(t, authServer)
		require.Len(t, sessions, 3)
		require.ElementsMatch(t, []string{"alice", "bob", "charlie"}, getAppSessionNames(t, sessions))
		require.Empty(t, emitter.Events())

		// First sweep.
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		sessions = mustListAppSessions(t, authServer)
		require.Len(t, sessions, 2)
		require.ElementsMatch(t, []string{"bob", "charlie"}, getAppSessionNames(t, sessions))
		require.Len(t, emitter.Events(), 1)

		// Stop just before the second sweep.
		time.Sleep(testInterval - time.Nanosecond)
		synctest.Wait()
		sessions = mustListAppSessions(t, authServer)
		require.Len(t, sessions, 2)
		require.ElementsMatch(t, []string{"bob", "charlie"}, getAppSessionNames(t, sessions))
		require.Len(t, emitter.Events(), 1)

		// Second sweep.
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		sessions = mustListAppSessions(t, authServer)
		require.Len(t, sessions, 1)
		require.Equal(t, "charlie", sessions[0].GetUser())
		require.Len(t, emitter.Events(), 2)

		// Stop just before the third sweep.
		time.Sleep(testInterval - time.Nanosecond)
		synctest.Wait()
		sessions = mustListAppSessions(t, authServer)
		require.Len(t, sessions, 1)
		require.Equal(t, "charlie", sessions[0].GetUser())
		require.Len(t, emitter.Events(), 2)

		// Third sweep.
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		require.Empty(t, mustListAppSessions(t, authServer))
		require.Len(t, emitter.Events(), 3)
	})
}

func setupExpiryService(t *testing.T) (*Service, *auth.Server, *eventstest.MockRecorderEmitter) {
	t.Helper()

	logger := logtest.NewLogger()
	emitter := &eventstest.MockRecorderEmitter{}

	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
		AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
			SecondFactor: constants.SecondFactorOn,
			Webauthn: &types.Webauthn{
				RPID: "localhost",
			},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { authServer.Close() })

	expiry, err := New(&Config{
		Log:         logger,
		Emitter:     emitter,
		AccessPoint: authServer.AuthServer,
	})
	require.NoError(t, err)

	return expiry, authServer.AuthServer, emitter
}

func createAccessRequest(t *testing.T, auth *auth.Server, state types.RequestState, expiry time.Time) types.AccessRequest {
	t.Helper()
	ctx := t.Context()

	req, err := types.NewAccessRequest(uuid.NewString(), "alice", "test_role_1")
	require.NoError(t, err)
	req.SetExpiry(expiry)
	req.SetState(state)

	err = auth.CreateAccessRequest(ctx, req)
	require.NoError(t, err)

	return req
}

func createAppSession(t *testing.T, authServer *auth.Server, user string, expiry time.Time) types.WebSession {
	t.Helper()
	ctx := t.Context()

	createdUser, _, err := authtest.CreateUserAndRole(authServer, user, []string{user}, nil)
	require.NoError(t, err)

	clusterName, err := authServer.GetClusterName(ctx)
	require.NoError(t, err)

	session, err := authServer.CreateAppSessionFromReq(ctx, auth.NewAppSessionRequest{
		NewWebSessionRequest: auth.NewWebSessionRequest{
			User:       user,
			Roles:      createdUser.GetRoles(),
			Traits:     createdUser.GetTraits(),
			SessionTTL: time.Until(expiry),
		},
		AppName:     "test-app.example.com",
		AppURI:      "https://test-app.example.com",
		PublicAddr:  "test-app.example.com",
		ClusterName: clusterName.GetClusterName(),
	})
	require.NoError(t, err)

	return session
}

func mustListAccessRequests(t *testing.T, auth *auth.Server) []*types.AccessRequestV3 {
	t.Helper()
	ctx := t.Context()

	resp, err := auth.Services.ListAccessRequests(ctx, &proto.ListAccessRequestsRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.NextKey)

	return resp.AccessRequests
}

func mustListAppSessions(t *testing.T, auth *auth.Server) []types.WebSession {
	t.Helper()
	ctx := t.Context()

	resp, _, err := auth.Services.ListAppSessions(ctx, 100, "", "")
	require.NoError(t, err)

	return resp
}

// Helper to extract session names from slice of WebSessions
func getAppSessionNames(t *testing.T, sessions []types.WebSession) []string {
	t.Helper()
	names := make([]string, 0, len(sessions))
	for _, s := range sessions {
		names = append(names, s.GetUser())
	}
	return names
}
