package provisioning

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
)

func TestSCIMClientWithBreakerSCIMAuthErrorsTripBreakerAndRecover(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	scimServer, scimUsers := newTestSCIMUsersServer(t)
	// Start with downstream SCIM auth failures so repeated requests can trip the breaker.
	scimUsers.setListUsersResponse(http.StatusUnauthorized)

	client := newTestProvisioningSCIMClient(t, clock, scimServer.URL+"/scim/v2", scimServer.Client())

	// Expect repeated auth failures to trip the breaker and stop new SCIM calls
	// from reaching the downstream server.
	scimCallsBeforeTrip := tripSCIMAuthBreaker(t, client, scimUsers)
	require.Equal(t, scimCallsBeforeTrip, scimUsers.listUsersCalls())

	// Restore the downstream SCIM response so the recovery probe can succeed
	// once the breaker allows traffic again.
	scimUsers.setListUsersResponse(http.StatusOK, &scimsdk.User{
		ID:       "user1",
		UserName: "user_one",
	})
	// Move past the tripped window so the breaker permits a recovery attempt.
	clock.Advance(authBreakerTrippedPeriod + time.Millisecond)

	// Expect the first call after the tripped period to reach the server and succeed.
	_, err := client.GetUserByUserName(t.Context(), "user_one")
	require.NoError(t, err)
	require.Equal(t, scimCallsBeforeTrip+1, scimUsers.listUsersCalls())
}

func TestSCIMClientWithBreakerSCIMNonAuthErrorsDoNotTripBreaker(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	scimServer, scimUsers := newTestSCIMUsersServer(t)
	// Start with a non-auth SCIM error to verify the breaker stays in standby.
	scimUsers.setListUsersResponse(http.StatusBadRequest)

	client := newTestProvisioningSCIMClient(t, clock, scimServer.URL+"/scim/v2", scimServer.Client())

	attemptsWithoutTrip := authBreakerFailuresBeforeTrip + 2
	for i := 0; i < attemptsWithoutTrip; i++ {
		// Expect non-auth SCIM errors to surface directly without tripping the breaker.
		_, err := client.GetUserByUserName(t.Context(), "user_one")
		require.True(t, trace.IsBadParameter(err))
		require.NotErrorIs(t, err, breaker.ErrStateTripped)
	}

	// Expect every attempt to still reach the downstream SCIM server.
	require.Equal(t, attemptsWithoutTrip, scimUsers.listUsersCalls())
}

func newTestProvisioningSCIMClient(t *testing.T, clock clockwork.Clock, endpoint string, httpClient *http.Client) scimsdk.Client {
	t.Helper()

	scimClientWithBreaker, err := NewSCIMClientWithBreaker(SCIMClientWithBreakerConfig{
		SCIMConfig: scimsdk.Config{
			Endpoint:        endpoint,
			Token:           "token",
			Log:             discardProvisioningLogger(),
			IntegrationType: "test",
			HTTPClient:      httpClient,
		},
		Clock: clock,
	})
	require.NoError(t, err)

	return scimClientWithBreaker
}

func tripSCIMAuthBreaker(t *testing.T, client scimsdk.Client, scimUsers *testSCIMUsersServer) int {
	t.Helper()

	lastObservedCalls := 0
	maxAttemptsBeforeTrip := authBreakerFailuresBeforeTrip + 2
	for attempt := 0; attempt < maxAttemptsBeforeTrip; attempt++ {
		_, err := client.GetUserByUserName(t.Context(), "user_one")
		if errors.Is(err, breaker.ErrStateTripped) {
			// Expect the tripping attempt to be rejected locally rather than sent downstream.
			return lastObservedCalls
		}
		// Expect pre-trip auth failures to surface as access denied.
		require.True(t, trace.IsAccessDenied(err))
		lastObservedCalls = scimUsers.listUsersCalls()
	}

	require.FailNow(t, "SCIM auth breaker did not trip")
	return 0
}

type testSCIMUsersServer struct {
	mu                  sync.Mutex
	listUsersStatusCode int
	listUsersUsers      []*scimsdk.User
	listUsersCallsCount int
}

func newTestSCIMUsersServer(t *testing.T) (*httptest.Server, *testSCIMUsersServer) {
	t.Helper()

	state := &testSCIMUsersServer{
		listUsersStatusCode: http.StatusOK,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/scim/v2/Users", state.handleListUsers)

	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)

	return server, state
}

func (s *testSCIMUsersServer) setListUsersResponse(statusCode int, users ...*scimsdk.User) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.listUsersStatusCode = statusCode
	s.listUsersUsers = users
}

func (s *testSCIMUsersServer) listUsersCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.listUsersCallsCount
}

func discardProvisioningLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func (s *testSCIMUsersServer) handleListUsers(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.listUsersCallsCount++

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if s.listUsersStatusCode != http.StatusOK {
		payload, err := scimsdk.FormatErrorResponse(s.listUsersStatusCode, "simulated SCIM failure")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(s.listUsersStatusCode)
		_, _ = w.Write(payload)
		return
	}

	resp := &scimsdk.ListUserResponse{
		Schemas:      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		TotalResults: int32(len(s.listUsersUsers)),
		StartIndex:   1,
		ItemsPerPage: int32(len(s.listUsersUsers)),
		Users:        s.listUsersUsers,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
