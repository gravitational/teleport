package oktaapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func requireNotFound(t require.TestingT, err error, _ ...any) {
	require.True(t, trace.IsNotFound(err), "Expected NotFound, got %s", err)
}

func TestErrorConversion(t *testing.T) {
	errorCases := []struct {
		name        string
		roundTrip   func(request *http.Request) (*http.Response, error)
		assertError require.ErrorAssertionFunc
	}{
		{
			name:        "Not Found",
			assertError: requireNotFound,
			roundTrip: func(request *http.Request) (*http.Response, error) {
				response := simulateOktaError(
					http.StatusNotFound,
					OktaErrCodeResourceNotFoundException,
					"Not Found: ResourceNotFound",
				)
				return response, nil
			},
		},

		{
			name:        "Access Denied",
			assertError: requireAccessDenied,
			roundTrip: func(request *http.Request) (*http.Response, error) {
				response := simulateOktaError(
					http.StatusUnauthorized,
					OktaErrCodeAccessDeniedException,
					"Access Denied",
				)
				return response, nil
			},
		},

		{
			name:        "Invalid Session",
			assertError: requireAccessDenied,
			roundTrip: func(request *http.Request) (*http.Response, error) {
				response := simulateOktaError(
					http.StatusForbidden,
					OktaErrCodeInvalidSessionException,
					"Invalid Session",
				)
				return response, nil
			},
		},

		{
			name:        "Invalid Token",
			assertError: requireAccessDenied,
			roundTrip: func(request *http.Request) (*http.Response, error) {
				response := simulateOktaError(
					http.StatusForbidden,
					OktaErrCodeInvalidTokenProvidedException,
					"Invalid token provided",
				)
				return response, nil
			},
		},

		{
			name:        "Misc",
			assertError: requireUrlError,
			roundTrip: func(request *http.Request) (*http.Response, error) {
				return nil, errors.New("some transient error")
			},
		},
	}

	operations := []struct {
		name   string
		action func(context.Context, Interface) error
	}{
		{
			name: "getCurrentUser",
			action: func(ctx context.Context, client Interface) error {
				_, err := client.GetCurrentUser(ctx)
				return err
			},
		},
		{
			name: "getGroupAssignments",
			action: func(ctx context.Context, client Interface) error {
				_, err := client.GetGroupAssignments(ctx, "someGroupID")
				return err
			},
		},
		{
			name: "getAppAssignments",
			action: func(ctx context.Context, client Interface) error {
				_, err := client.GetAppAssignments(ctx, "someAppID")
				return err
			},
		},
		{
			name: "getAppGroups",
			action: func(ctx context.Context, client Interface) error {
				_, err := client.GetAppGroups(ctx, "someAppID")
				return err
			},
		},
		{
			name: "listUsers",
			action: func(ctx context.Context, client Interface) error {
				_, err := client.ListUsers(ctx)
				return err
			},
		},
		{
			name: "assignUserToGroup",
			action: func(ctx context.Context, client Interface) error {
				return client.AssignUserToGroup(ctx, "someUserID", "someGroupID")
			},
		},
		{
			name: "unassignUserFromGroup",
			action: func(ctx context.Context, client Interface) error {
				return client.UnassignUserFromGroup(ctx, "someUserID", "someGroupID")
			},
		},
		{
			name: "assignUserToApplication",
			action: func(ctx context.Context, client Interface) error {
				return client.AssignUserToApplication(ctx, "someUserID", "someAppID")
			},
		},
		{
			name: "assignGroupToApplication",
			action: func(ctx context.Context, client Interface) error {
				return client.AssignGroupToApplication(ctx, "someGroupID", "someAppID")
			},
		},
		{
			name: "unassignUserFromApplication",
			action: func(ctx context.Context, client Interface) error {
				return client.UnassignUserFromApplication(ctx, "someGroupID", "someAppID")
			},
		},
		{
			name: "getApplication",
			action: func(ctx context.Context, client Interface) error {
				var app okta.App
				_, err := client.GetApplication(ctx, "someAppID", app)
				return err
			},
		},
	}

	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {
			for _, ec := range errorCases {
				t.Run(ec.name, func(t *testing.T) {
					testCtx, cancel := context.WithCancel(context.Background())
					t.Cleanup(cancel)

					mockta := &mockRoundTripper{}
					mockta.
						On("RoundTrip", anyRequest).
						Return(ec.roundTrip)

					client, err := New(testCtx, Config{
						OrgUrl:         "https://okta.example.com",
						AuthProvider:   NewSSWSAuthProvider("i-am-not-a-token"),
						TestHTTPClient: &http.Client{Transport: mockta},
						Log:            slog.With("test", t.Name()),
					})
					require.NoError(t, err)

					ec.assertError(t, op.action(testCtx, client))
				})
			}

		})
	}
}

func requireAccessDenied(t require.TestingT, err error, i ...any) {
	require.True(t, trace.IsAccessDenied(err), "err should be access denied, was: %s", err)
}

func requireUrlError(t require.TestingT, err error, _ ...any) {
	require.Error(t, err)

	var urlErr *url.Error
	require.ErrorAs(t, err, &urlErr)
}

func simulateOktaError(httpStatus int, errorCode string, summary string) *http.Response {
	oktaErr := okta.Error{
		ErrorCode:    errorCode,
		ErrorSummary: summary,
		ErrorLink:    errorCode,
		ErrorId:      "someRandomString",
		ErrorCauses:  []map[string]any{},
	}

	body, err := json.Marshal(&oktaErr)
	if err != nil {
		panic(err)
	}

	r := &http.Response{
		StatusCode: httpStatus,
		Status:     http.StatusText(httpStatus),
		Header:     make(http.Header),
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", strconv.Itoa(len(body)))
	r.Body = io.NopCloser(bytes.NewReader(body))
	return r
}

var anyRequest any = mock.MatchedBy(func(req *http.Request) bool { return true })

type mockRoundTripper struct {
	mock.Mock
}

// RoundTrip implements the RoundTripper interface for the mockRoundTripper
func (m *mockRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	args := m.Called(request)

	// Sometimes we want to invoke a function and return the result of that
	// function as the mock call result. Testify doesn't let us do that out of
	// the box but, by convention, we simulate it by allowing the test to supply
	// a function with the same signature as the mocked-put method as a `Return()`
	// value.
	fn, isDelegate := args.Get(0).(func(request *http.Request) (*http.Response, error))
	if isDelegate {
		return fn(request)
	}

	maybeResponse := args.Get(0)
	if maybeResponse == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}
