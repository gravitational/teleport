// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package msgraph

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/msgraph/models"
	"github.com/gravitational/teleport/lib/msgraph/msgraphtest"
)

// Always sleep for a second for predictability
var retryConfig = retryutils.RetryV2Config{
	First:  time.Second,
	Max:    time.Second,
	Driver: retryutils.NewLinearDriver(time.Second),
}

type fakeTokenProvider struct {
	mu    sync.Mutex
	token string
}

func (t *fakeTokenProvider) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.token == "" {
		t.token = uuid.NewString()
	}

	return azcore.AccessToken{
		Token: t.token,
	}, nil
}

func (t *fakeTokenProvider) clearToken() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.token = ""
}

// inspectToken returns the current token without generating a new one if the current token is
// empty. Useful in tests that need to verify that the client requested a new token after it was
// cleared.
func (t *fakeTokenProvider) inspectToken() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.token
}

const usersPayload = `[
    {
    	"businessPhones": [],
    	"displayName": "Alice Alison",
    	"givenName": null,
    	"jobTitle": null,
    	"mail": "alice@example.com",
    	"mobilePhone": null,
    	"officeLocation": null,
    	"preferredLanguage": null,
    	"surname": null,
    	"userPrincipalName": "alice@example.com",
    	"id": "6e7b768e-07e2-4810-8459-485f84f8f204"
    },
    {
    	"businessPhones": [
    		"+1 425 555 0109"
    	],
    	"displayName": "Bob Bobert",
    	"givenName": "Bob",
    	"jobTitle": "Product Marketing Manager",
    	"mail": "bob@example.com",
    	"mobilePhone": null,
    	"officeLocation": "18/2111",
    	"preferredLanguage": "en-US",
    	"surname": "Bobert",
    	"userPrincipalName": "bob@example.com",
    	"id": "87d349ed-44d7-43e1-9a83-5f2406dee5bd"
    },
    {
    	"businessPhones": [
    		"8006427676"
    	],
    	"displayName": "Administrator",
    	"givenName": null,
    	"jobTitle": null,
    	"mail": "admin@example.com",
    	"mobilePhone": "5555555555",
    	"officeLocation": null,
    	"preferredLanguage": "en-US",
    	"surname": null,
		"onPremisesSamAccountName": "AD Administrator",
    	"userPrincipalName": "admin@example.com",
    	"id": "5bde3e51-d13b-4db1-9948-fe4b109d11a7"
    },
    {
    	"businessPhones": [
    		"+1 858 555 0110"
    	],
    	"displayName": "Carol C",
    	"givenName": "Carol",
    	"jobTitle": "Marketing Assistant",
    	"mail": "carol@example.com",
    	"mobilePhone": null,
    	"officeLocation": "131/1104",
    	"preferredLanguage": "en-US",
    	"surname": "C",
    	"userPrincipalName": "carol@example.com",
    	"id": "4782e723-f4f4-4af3-a76e-25e3bab0d896"
    },
    {
    	"businessPhones": [
    		"+1 262 555 0106"
    	],
    	"displayName": "Eve Evil",
    	"givenName": "Eve",
    	"jobTitle": "Corporate Security Officer",
    	"mail": "eve@example.com",
    	"mobilePhone": null,
    	"officeLocation": "24/1106",
    	"preferredLanguage": "en-US",
    	"surname": "Evil",
    	"userPrincipalName": "eve#EXT#@example.com",
    	"id": "c03e6eaa-b6ab-46d7-905b-73ec7ea1f755"
    }
]`

// paginatedHandler emulates the Graph API's pagination with the given static set of objects.
func paginatedHandler(t *testing.T, values []json.RawMessage) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		top, err := strconv.Atoi(r.URL.Query().Get("$top"))
		if err != nil {
			assert.Fail(t, "expected to get $top parameter")
		}
		skip, _ := strconv.Atoi(r.URL.Query().Get("$skipToken"))

		from, to := skip, skip+top
		if to > len(values) {
			to = len(values)
		}
		page := values[from:to]

		nextLink := *r.URL
		nextLink.Host = r.Host
		nextLink.Scheme = "https"
		vals := nextLink.Query()
		// $skipToken is an opaque value in MS Graph, for testing purposes we use a simple offset.
		vals.Set("$skipToken", strconv.Itoa(top+skip))
		nextLink.RawQuery = vals.Encode()

		response := map[string]any{
			"value": page,
		}
		if skip+top < len(values) {
			response["@odata.nextLink"] = nextLink.String()
		}
		assert.NoError(t, json.NewEncoder(w).Encode(&response))
	})
}

func TestIterateUsers_Empty(t *testing.T) {
	t.Parallel()

	storage := msgraphtest.NewDefaultStorage()
	// overwrite user storage to test empty user response.
	storage.Users = make(map[string]*models.User)
	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(storage))

	t.Cleanup(func() { fakeServer.TLSServer.Close() })

	client, err := NewClient(Config{
		HTTPClient:    newHTTPClient(fakeServer.TLSServer),
		TokenProvider: &fakeTokenProvider{},
		RetryConfig:   &retryConfig,
	})
	require.NoError(t, err)
	err = client.IterateUsers(t.Context(), func(*models.User) bool {
		assert.Fail(t, "should never get called")
		return true
	})
	require.NoError(t, err)
}

func TestIterateUsers(t *testing.T) {
	t.Parallel()

	var sourceUsers []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(usersPayload), &sourceUsers))
	mux := http.NewServeMux()
	mux.Handle("GET /v1.0/users", paginatedHandler(t, sourceUsers))

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(func() { srv.Close() })

	client, err := NewClient(Config{
		HTTPClient:    newHTTPClient(srv),
		TokenProvider: &fakeTokenProvider{},
		RetryConfig:   &retryConfig,
		PageSize:      2, // smaller page size so we actually fetch multiple pages with our small test payload
	})
	require.NoError(t, err)

	var users []*models.User
	err = client.IterateUsers(t.Context(), func(u *models.User) bool {
		users = append(users, u)
		return true
	})

	require.NoError(t, err)
	require.Len(t, users, 5)

	require.Equal(t, "6e7b768e-07e2-4810-8459-485f84f8f204", *users[0].ID)
	require.Equal(t, "alice@example.com", *users[0].Mail)
	require.Equal(t, "Alice Alison", *users[0].DisplayName)
	require.Equal(t, "alice@example.com", *users[0].UserPrincipalName)
	require.Nil(t, users[0].Surname)
	require.Nil(t, users[0].GivenName)

	require.Equal(t, "bob@example.com", *users[1].Mail)
	require.Equal(t, "bob@example.com", *users[1].UserPrincipalName)
	require.Equal(t, "Bobert", *users[1].Surname)
	require.Equal(t, "Bob", *users[1].GivenName)

	require.Equal(t, "admin@example.com", *users[2].Mail)
	require.Equal(t, "admin@example.com", *users[2].UserPrincipalName)
	require.Equal(t, "AD Administrator", *users[2].OnPremisesSAMAccountName)

	require.Equal(t, "carol@example.com", *users[3].Mail)
	require.Equal(t, "carol@example.com", *users[3].UserPrincipalName)

	require.Equal(t, "eve@example.com", *users[4].Mail)
	require.Equal(t, "eve#EXT#@example.com", *users[4].UserPrincipalName)
}

type failingHandler struct {
	t              *testing.T
	timesCalled    atomic.Int32
	timesToFail    int32
	statusCode     int
	expectedBody   []byte
	successPayload []byte
	retryAfter     int
}

func (f *failingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.expectedBody != nil {
		body, err := io.ReadAll(r.Body)
		assert.NoError(f.t, err)
		assert.Equal(f.t, f.expectedBody, body)
		r.Body.Close()
	}
	if f.retryAfter != 0 {
		w.Header().Add("Retry-After", strconv.Itoa(f.retryAfter))
	}
	if f.timesCalled.Load() < f.timesToFail {
		w.WriteHeader(f.statusCode)
		w.Write([]byte("{}"))
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write(f.successPayload)
	}
	f.timesCalled.Add(1)
}

func TestRetry(t *testing.T) {
	t.Parallel()

	appID := uuid.NewString()
	route := "POST /v1.0/applications/" + appID + "/federatedIdentityCredentials"
	name := "foo"
	fic := &models.FederatedIdentityCredential{Name: &name}
	objPayload, err := json.Marshal(fic)
	require.NoError(t, err)

	clock := clockwork.NewFakeClock()

	t.Run("retriable, with retry-after", func(t *testing.T) {
		handler := &failingHandler{
			t:              t,
			timesToFail:    2,
			statusCode:     http.StatusTooManyRequests,
			expectedBody:   objPayload,
			successPayload: objPayload,
			retryAfter:     10,
		}
		mux := http.NewServeMux()
		mux.Handle(route, handler)

		srv := httptest.NewTLSServer(mux)
		t.Cleanup(func() { srv.Close() })

		client, err := NewClient(Config{
			HTTPClient:    newHTTPClient(srv),
			TokenProvider: &fakeTokenProvider{},
			RetryConfig:   &retryConfig,
			Clock:         clock,
		})
		require.NoError(t, err)

		ret := make(chan error, 1)
		go func() {
			out, err := client.CreateFederatedIdentityCredential(t.Context(), appID, fic)
			assert.Equal(t, fic, out)
			ret <- err
		}()

		// Fail for the first time
		clock.BlockUntilContext(t.Context(), 1)
		require.EqualValues(t, 1, handler.timesCalled.Load())

		// Fail for the second time
		clock.Advance(time.Duration(handler.retryAfter) * time.Second)
		clock.BlockUntilContext(t.Context(), 1)
		require.EqualValues(t, 2, handler.timesCalled.Load())

		// Succeed
		clock.Advance(time.Duration(handler.retryAfter) * time.Second)
		select {
		case err := <-ret:
			require.NoError(t, err)
		case <-time.After(time.Second):
			require.Fail(t, "expected client to return")

		}
	})

	t.Run("retriable, without retry-after", func(t *testing.T) {
		handler := &failingHandler{
			t:              t,
			timesToFail:    2,
			statusCode:     http.StatusTooManyRequests,
			expectedBody:   objPayload,
			successPayload: objPayload,
		}
		mux := http.NewServeMux()
		mux.Handle(route, handler)

		srv := httptest.NewTLSServer(mux)
		t.Cleanup(func() { srv.Close() })

		client, err := NewClient(Config{
			HTTPClient:    newHTTPClient(srv),
			TokenProvider: &fakeTokenProvider{},
			RetryConfig:   &retryConfig,
			PageSize:      2, // smaller page size so we actually fetch multiple pages with our small test payload
			Clock:         clock,
		})
		require.NoError(t, err)

		ret := make(chan error, 1)
		go func() {
			out, err := client.CreateFederatedIdentityCredential(t.Context(), appID, fic)
			assert.Equal(t, fic, out)
			ret <- err
		}()

		// Fail for the first time
		clock.BlockUntilContext(t.Context(), 1)
		require.EqualValues(t, 1, handler.timesCalled.Load())

		// Fail for the second time
		clock.Advance(time.Second)
		clock.BlockUntilContext(t.Context(), 1)
		require.EqualValues(t, 2, handler.timesCalled.Load())

		// Succeed
		clock.Advance(time.Second)
		select {
		case err := <-ret:
			require.NoError(t, err)
		case <-time.After(time.Second):
			require.Fail(t, "expected client to return")

		}
	})

	t.Run("non-retriable", func(t *testing.T) {
		handler := &failingHandler{
			t:            t,
			timesToFail:  1,
			statusCode:   http.StatusNotFound,
			expectedBody: objPayload,
		}
		mux := http.NewServeMux()
		mux.Handle(route, handler)

		srv := httptest.NewTLSServer(mux)
		t.Cleanup(func() { srv.Close() })

		client, err := NewClient(Config{
			HTTPClient:    newHTTPClient(srv),
			TokenProvider: &fakeTokenProvider{},
			RetryConfig:   &retryConfig,
			Clock:         clock,
		})
		require.NoError(t, err)

		_, err = client.CreateFederatedIdentityCredential(t.Context(), appID, fic)
		require.Error(t, err)
	})

	// This test simulates a situation in which the token expires between retries. It verifies that
	// the client requests a token before each retry rather than requesting it just once before it
	// enters the retry loop.
	t.Run("refreshing token between retries", func(t *testing.T) {
		handler := &failingHandler{
			t:              t,
			timesToFail:    1,
			statusCode:     http.StatusTooManyRequests,
			expectedBody:   objPayload,
			successPayload: objPayload,
			retryAfter:     10,
		}
		mux := http.NewServeMux()
		mux.Handle(route, handler)

		srv := httptest.NewTLSServer(mux)
		t.Cleanup(func() { srv.Close() })

		tokenProvider := &fakeTokenProvider{}
		client, err := NewClient(Config{
			HTTPClient:    newHTTPClient(srv),
			TokenProvider: tokenProvider,
			Clock:         clock,
			RetryConfig:   &retryConfig,
		})
		require.NoError(t, err)

		ret := make(chan error, 1)
		go func() {
			out, err := client.CreateFederatedIdentityCredential(context.Background(), appID, fic)
			assert.Equal(t, fic, out)
			ret <- err
		}()

		// First failure, the client now waits before retrying.
		require.NoError(t, clock.BlockUntilContext(t.Context(), 1))
		require.EqualValues(t, 1, handler.timesCalled.Load())
		tokenBefore := tokenProvider.inspectToken()
		require.NotEmpty(t, tokenBefore)

		// Clear the token to simulate expiry.
		tokenProvider.clearToken()

		// Advance time to make the client try again.
		clock.Advance(time.Duration(handler.retryAfter) * time.Second)
		select {
		case err := <-ret:
			require.NoError(t, err)
		case <-time.After(time.Second):
			require.Fail(t, "expected client to return")
		}

		tokenAfter := tokenProvider.inspectToken()
		require.NotEmpty(t, tokenAfter,
			"the client did not request a new token after the previous one was cleared")
		require.NotEqual(t, tokenAfter, tokenBefore,
			"the client did not get a new token for the second request")
	})
}

const listGroupsMembersPayload = `[
    {
      "@odata.type": "#microsoft.graph.user",
      "id": "9f615773-8219-4a5e-9eb1-8e701324c683",
      "mail": "alice@example.com"
    },
	{
      "@odata.type": "#microsoft.graph.device",
      "id": "1566d9a7-c652-44e7-a75e-665b77431435",
      "mail": "device@example.com"
    },
	{
      "@odata.type": "#microsoft.graph.group",
      "id": "7db727c5-924a-4f6d-b1f0-d44e6cafa87c",
      "displayName": "Test Group 1"
    }
  ]`

func TestIterateGroupMembers(t *testing.T) {
	t.Parallel()

	var membersJSON []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(listGroupsMembersPayload), &membersJSON))
	mux := http.NewServeMux()
	groupID := "fd5be192-6e51-4f54-bbdf-30407435ceb7"
	mux.Handle("GET /v1.0/groups/"+groupID+"/members", paginatedHandler(t, membersJSON))

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(func() { srv.Close() })

	client, err := NewClient(Config{
		HTTPClient:    newHTTPClient(srv),
		TokenProvider: &fakeTokenProvider{},
		RetryConfig:   &retryConfig,
		PageSize:      2, // smaller page size so we actually fetch multiple pages with our small test payload
	})
	require.NoError(t, err)

	var members []models.GroupMember
	err = client.IterateGroupMembers(t.Context(), groupID, func(u models.GroupMember) bool {
		members = append(members, u)
		return true
	})

	require.NoError(t, err)
	require.Len(t, members, 2)
	{
		require.IsType(t, &models.User{}, members[0])
		user := members[0].(*models.User)
		require.Equal(t, "9f615773-8219-4a5e-9eb1-8e701324c683", *user.ID)
		require.Equal(t, "alice@example.com", *user.Mail)
	}
	{
		require.IsType(t, &models.Group{}, members[1])
		group := members[1].(*models.Group)
		require.Equal(t, "7db727c5-924a-4f6d-b1f0-d44e6cafa87c", *group.ID)
		require.Equal(t, "Test Group 1", *group.DisplayName)
	}
}

const getApplicationPayload = `
{
        "id": "aeee7e9f-57ad-4ea6-a236-cd10b2dbc0b4",
        "appId": "d2a39a2a-1636-457f-82f9-c2d76527e20e",
        "displayName": "test SAML App",
        "groupMembershipClaims": "SecurityGroup",
        "identifierUris": [
            "goteleport.com"
        ],
        "optionalClaims": {
            "accessToken": [],
            "idToken": [],
            "saml2Token": [
                {
                    "additionalProperties": [
                        "sam_account_name"
                    ],
                    "essential": false,
                    "name": "groups",
                    "source": null
                }
            ]
        }
    }`

func TestGetApplication(t *testing.T) {

	mux := http.NewServeMux()
	appID := "d2a39a2a-1636-457f-82f9-c2d76527e20e"
	mux.Handle(fmt.Sprintf("GET /v1.0/applications(appId='%s')", appID),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(getApplicationPayload))
		}))

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(func() { srv.Close() })

	client, err := NewClient(Config{
		TokenProvider: &fakeTokenProvider{},
		HTTPClient:    newHTTPClient(srv),
		RetryConfig:   &retryConfig,
		PageSize:      2, // smaller page size so we actually fetch multiple pages with our small test payload
	})
	require.NoError(t, err)

	app, err := client.GetApplication(t.Context(), appID)
	require.NoError(t, err)
	require.Equal(t, "aeee7e9f-57ad-4ea6-a236-cd10b2dbc0b4", *app.ID)

	expectation := &models.Application{
		AppID: toPtr("d2a39a2a-1636-457f-82f9-c2d76527e20e"),
		DirectoryObject: models.DirectoryObject{
			DisplayName: toPtr("test SAML App"),
			ID:          toPtr("aeee7e9f-57ad-4ea6-a236-cd10b2dbc0b4"),
		},
		GroupMembershipClaims: toPtr("SecurityGroup"),
		IdentifierURIs:        &[]string{"goteleport.com"},
		OptionalClaims: &models.OptionalClaims{
			AccessToken: []models.OptionalClaim{},
			IDToken:     []models.OptionalClaim{},
			SAML2Token: []models.OptionalClaim{
				{
					AdditionalProperties: []string{"sam_account_name"},
					Essential:            toPtr(false),
					Name:                 toPtr("groups"),
					Source:               nil,
				},
			},
		},
	}
	require.EqualValues(t, expectation, app)

}

func toPtr[T any](s T) *T { return &s }

func TestNewClient(t *testing.T) {
	tests := []struct {
		name                  string
		config                Config
		expectedGraphEndpoint string
		errExpected           bool
		errAssertion          require.ErrorAssertionFunc
	}{
		{
			name: "empty endpoint sets default graph endpoint",
			config: Config{
				TokenProvider: &fakeTokenProvider{},
				GraphEndpoint: "",
			},
			expectedGraphEndpoint: types.MSGraphDefaultEndpoint,
			errAssertion:          require.NoError,
		},
		{
			name: "configured endpoint",
			config: Config{
				TokenProvider: &fakeTokenProvider{},
				GraphEndpoint: "https://dod-graph.microsoft.us",
			},
			expectedGraphEndpoint: "https://dod-graph.microsoft.us",
			errAssertion:          require.NoError,
		},
		{
			name: "invalid endpoint",
			config: Config{
				TokenProvider: &fakeTokenProvider{},
				GraphEndpoint: "https://graph.windows.net",
			},
			errExpected:  true,
			errAssertion: require.Error,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			clt, err := NewClient(test.config)
			test.errAssertion(t, err)
			if !test.errExpected {
				require.Equal(t, test.expectedGraphEndpoint+"/"+graphVersion, clt.baseURL.String())
			}
		})
	}
}

func TestIterateUsersTransitiveMemberOf(t *testing.T) {
	userID := "9ef1bc41-1b26-4a66-b8bc-956b2a54f8dc"
	allGroupsPath := fmt.Sprintf("/%s/users/%s/transitiveMemberOf", graphVersion, userID)
	groupsPath := fmt.Sprintf("/%s/users/%s/transitiveMemberOf/%s", graphVersion, userID, graphNamespaceGroups)
	directoryRolePath := fmt.Sprintf("/%s/users/%s/transitiveMemberOf/%s", graphVersion, userID, graphNamespaceDirectoryRoles)

	consistencyHeaderValue := ""
	foundQuery := make(url.Values)
	requestedPath := ""
	withRequestChecker := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPath = r.URL.Path
			consistencyHeaderValue = r.Header.Get("ConsistencyLevel")
			foundQuery = r.URL.Query()
			next.ServeHTTP(w, r)
		})
	}

	mux := http.NewServeMux()
	var groups []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(userGroups), &groups))
	mux.Handle("GET "+allGroupsPath, withRequestChecker(paginatedHandler(t, groups)))
	mux.Handle("GET "+groupsPath, withRequestChecker(paginatedHandler(t, groups)))
	mux.Handle("GET "+directoryRolePath, withRequestChecker(paginatedHandler(t, groups)))
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(func() { srv.Close() })

	client, err := NewClient(Config{
		HTTPClient:    newHTTPClient(srv),
		TokenProvider: &fakeTokenProvider{},
		RetryConfig:   &retryConfig,
		PageSize:      2, // smaller page size so we actually fetch multiple pages with our small test payload
	})
	require.NoError(t, err)

	assertConsistencyLevelHeader := func(t *testing.T, h string) {
		t.Helper()
		require.Equal(t, "eventual", h, "request made without ConsistencyLevel header")
	}
	assertCountQuery := func(t *testing.T, c string) {
		t.Helper()
		require.Equal(t, "true", c, "request made without $count query")
	}
	assertRequestedPath := func(t *testing.T, e, p string) {
		t.Helper()
		require.Equal(t, e, p, "expected request path did not match")
	}

	t.Run(types.EntraIDSecurityGroups, func(t *testing.T) {
		var groupIDs []string
		err := client.IterateUsersTransitiveMemberOf(t.Context(), userID, types.EntraIDSecurityGroups, func(group *models.Group) bool {
			groupIDs = append(groupIDs, *group.ID)
			return true
		})
		require.NoError(t, err)
		require.Len(t, groupIDs, 5)

		filterValue, err := url.QueryUnescape(foundQuery.Get("$filter"))
		require.NoError(t, err)
		require.Equal(t, securityGroupsFilter, filterValue, "security groups request made without filter query")
		assertConsistencyLevelHeader(t, consistencyHeaderValue)
		assertRequestedPath(t, groupsPath, requestedPath)
		assertCountQuery(t, foundQuery.Get("$count"))
	})

	t.Run(types.EntraIDAllGroups, func(t *testing.T) {
		var groupIDs []string
		err := client.IterateUsersTransitiveMemberOf(t.Context(), userID, types.EntraIDAllGroups, func(group *models.Group) bool {
			groupIDs = append(groupIDs, *group.ID)
			return true
		})
		require.NoError(t, err)
		require.Len(t, groupIDs, 5)

		require.Empty(t, foundQuery.Get("$filter"), "non security groups request made with filter query")
		assertConsistencyLevelHeader(t, consistencyHeaderValue)
		assertRequestedPath(t, allGroupsPath, requestedPath)
		assertCountQuery(t, foundQuery.Get("$count"))
	})

	t.Run(types.EntraIDDirectoryRoles, func(t *testing.T) {
		var groupIDs []string
		err := client.IterateUsersTransitiveMemberOf(t.Context(), userID, types.EntraIDDirectoryRoles, func(group *models.Group) bool {
			groupIDs = append(groupIDs, *group.ID)
			return true
		})
		require.NoError(t, err)
		require.Len(t, groupIDs, 5)

		require.Empty(t, foundQuery.Get("$filter"), "non security groups request made with filter query")
		assertConsistencyLevelHeader(t, consistencyHeaderValue)
		assertRequestedPath(t, directoryRolePath, requestedPath)
		assertCountQuery(t, foundQuery.Get("$count"))
	})

	t.Run("unsupported-group-type", func(t *testing.T) {
		var groupIDs []string
		err := client.IterateUsersTransitiveMemberOf(t.Context(), userID, "unsupported-group-type", func(group *models.Group) bool {
			groupIDs = append(groupIDs, *group.ID)
			return true
		})
		require.Error(t, err)
	})
}

var userGroups = `
[
	{
		"id": "07af5ddc-0f6b-4237-8b3c-64815501d1d5"
	},
	{
		"id": "dd034a93-4ac3-4095-8b9e-f521ad7eace9"
	},
	{
		"id": "20b81a2c-fda0-41e7-8268-48a014be0b08"
	},
	{
		"id": "97336101-e9a4-4455-9d19-945fd9178ff6"
	},
	{
		"id": "76c1db72-be9c-4ed5-8a42-bdeec6a34502"
	}
]
`

const listGroupsOwnersPayload = `[
    {
      "id": "9f615773-8219-4a5e-9eb1-8e701324c683",
      "userPrincipalName": "alice@example.com"
    },
	{
      "id": "1566d9a7-c652-44e7-a75e-665b77431435",
      "userPrincipalName": "bob@example.com"
    },
	{
      "id": "1566d9a7-c652-44e7-a75e-665b77431436",
      "userPrincipalName": "carol@example.com"
    }
  ]`

func TestIterateGroupOwners(t *testing.T) {
	t.Parallel()

	var ownersJSON []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(listGroupsOwnersPayload), &ownersJSON))
	mux := http.NewServeMux()
	groupID := "fd5be192-6e51-4f54-bbdf-30407435ceb7"
	mux.Handle("GET /v1.0/groups/{groupID}/owners/microsoft.graph.user", paginatedHandler(t, ownersJSON))

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	client, err := NewClient(Config{
		HTTPClient:    newHTTPClient(srv),
		TokenProvider: &fakeTokenProvider{},
		RetryConfig:   &retryConfig,
		PageSize:      2, // smaller page size so we actually fetch multiple pages with our small test payload
	})
	require.NoError(t, err)

	// owners are of User type.
	var owners []*models.User
	err = client.IterateGroupOwners(t.Context(), groupID, func(o *models.User) bool {
		owners = append(owners, o)
		return true
	})
	require.NoError(t, err)
	require.Len(t, owners, 3)

	expectedOwners := []string{"alice@example.com", "bob@example.com", "carol@example.com"}
	gotOwners := []string{}
	for _, o := range owners {
		gotOwners = append(gotOwners, *o.UserPrincipalName)
	}
	require.ElementsMatch(t, expectedOwners, gotOwners)
}

func newHTTPClient(server *httptest.Server) *http.Client {
	var d net.Dialer
	httpClient := server.Client()
	httpClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		// Ignore the address and always direct all requests to the fake API server.
		// This allows tests to connect to the fake API server despite the client trying to reach the
		// official endpoints.
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return d.DialContext(ctx, "tcp", server.Listener.Addr().String())
		},
	}
	return httpClient
}

func TestIterateUserDeltas(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	defaultStorage := msgraphtest.NewDefaultStorage()

	// Seed storage with one user carol.
	storage := msgraphtest.NewStorage()
	carol := defaultStorage.Users[msgraphtest.CarolID]
	storage.Users[*carol.ID] = carol

	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(storage))
	t.Cleanup(func() { fakeServer.TLSServer.Close() })

	client, err := NewClient(Config{
		HTTPClient:    newHTTPClient(fakeServer.TLSServer),
		TokenProvider: &fakeTokenProvider{},
		RetryConfig:   &retryConfig,
	})
	require.NoError(t, err)

	fakeDeltaStore := msgraphtest.NewFakeDeltaStore()
	const userEndpoint = "users/delta"

	// Request without the latest token should fail.
	for _, err := range client.IterateUserDeltas(ctx, userEndpoint, fakeDeltaStore) {
		require.ErrorIs(t, err, ErrMissingDeltaLink)
	}

	// Set up latest delta token for user endpoint.
	err = client.SetupLatestDelta(ctx, userEndpoint, fakeDeltaStore)
	require.NoError(t, err)

	// Subsequent delta requests should now succeed.
	require.NotEmpty(t, fakeDeltaStore.Get(userEndpoint))

	sortDeltas := cmpopts.SortSlices(func(a, b *models.ListUsersDeltaResponse) bool {
		return a.User.GetID() != nil &&
			b.User.GetID() != nil &&
			*a.User.GetID() < *b.User.GetID()
	})

	// Add alice and bob users.
	alice := defaultStorage.Users[msgraphtest.AliceID]
	bob := defaultStorage.Users[msgraphtest.BobID]
	fakeServer.SetUsers([]*models.User{alice, bob})
	expected := []*models.ListUsersDeltaResponse{
		{
			User: &models.User{
				DirectoryObject: models.DirectoryObject{
					ID: alice.GetID(),
				},
				Mail:              alice.Mail,
				UserPrincipalName: alice.UserPrincipalName,
			},
		},
		{
			User: &models.User{
				DirectoryObject: models.DirectoryObject{
					ID: bob.GetID(),
				},
				Mail:              bob.Mail,
				UserPrincipalName: bob.UserPrincipalName,
			},
		},
	}
	// Test new users response.
	got := []*models.ListUsersDeltaResponse{}
	for usersDelta, err := range client.IterateUserDeltas(ctx, userEndpoint, fakeDeltaStore) {
		require.NoError(t, err)

		got = append(got, usersDelta)
	}
	require.Empty(t, cmp.Diff(expected, got, sortDeltas), "expected user delta response to match")

	// Test user delete response
	fakeServer.DeleteUsers([]string{*carol.GetID()})
	expected = []*models.ListUsersDeltaResponse{
		{
			User: &models.User{
				DirectoryObject: models.DirectoryObject{
					ID: carol.GetID(),
				},
			},
			Removed: &models.RemovedReason{
				Reason: to.Ptr("deleted"),
			},
		},
	}
	got = []*models.ListUsersDeltaResponse{}
	for usersDelta, err := range client.IterateUserDeltas(ctx, userEndpoint, fakeDeltaStore) {
		require.NoError(t, err)

		got = append(got, usersDelta)
	}

	require.Empty(t, cmp.Diff(expected, got, sortDeltas), "expected user delta response to match")
}

func TestIterateGroupDeltas(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	defaultStorage := msgraphtest.NewDefaultStorage()

	// Start with default users alice, bob and carol.
	storage := msgraphtest.NewStorage()
	storage.Users = defaultStorage.Users

	fakeServer := msgraphtest.NewServer(msgraphtest.WithStorage(storage))
	t.Cleanup(func() { fakeServer.TLSServer.Close() })

	httpClient := &http.Client{
		Transport: &msgraphtest.RewriteTransport{
			Base: fakeServer.TLSServer.Client().Transport,
			URL:  mustParseURL(t, fakeServer.TLSServer.URL),
		},
	}

	client, err := NewClient(Config{
		HTTPClient:    httpClient,
		TokenProvider: &fakeTokenProvider{},
		RetryConfig:   &retryConfig,
	})
	require.NoError(t, err)

	fakeDeltaStore := msgraphtest.NewFakeDeltaStore()

	const endpoint = "groups/delta"

	// Request without the latest token should fail.
	for _, err := range client.IterateGroupDeltas(ctx, endpoint, fakeDeltaStore) {
		require.ErrorIs(t, err, ErrMissingDeltaLink)
	}

	// Set up latest delta token for user endpoint.
	err = client.SetupLatestDelta(ctx, endpoint, fakeDeltaStore, WithSelect("id,displayName,description,members,owners"))
	require.NoError(t, err)

	require.NotEmpty(t, fakeDeltaStore.Get(endpoint))

	sortDeltas := cmpopts.SortSlices(func(a, b *models.ListGroupsDeltaResponse) bool {
		return a.Group.GetID() != nil &&
			b.Group.GetID() != nil &&
			*a.Group.GetID() < *b.Group.GetID()
	})

	// Create groups
	group1 := defaultStorage.Groups[msgraphtest.Group1ID]
	group2 := defaultStorage.Groups[msgraphtest.Group2ID]
	group3 := defaultStorage.Groups[msgraphtest.Group3ID]
	fakeServer.SetGroups([]*models.Group{group1, group2, group3})
	// Add user alice and group3 as member
	alice := storage.Users[msgraphtest.AliceID]
	fakeServer.SetGroupMembers(*group1.GetID(), []models.GroupMember{alice, group3})
	expected := []*models.ListGroupsDeltaResponse{
		{
			Group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID:          group1.GetID(),
					DisplayName: group1.DisplayName,
				},
				GroupTypes: []string{"security-groups"},
			},
			Members: []models.MembersDelta{
				{
					DirectoryObject: &models.DirectoryObject{
						ID: alice.GetID(),
					},
					Type: models.ODataUser,
				},
				{
					DirectoryObject: &models.DirectoryObject{
						ID: group3.GetID(),
					},
					Type: models.ODataGroup,
				},
			},
		},
		{
			Group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID:          group2.GetID(),
					DisplayName: group2.DisplayName,
				},
				GroupTypes: []string{"security-groups"},
			},
		},
		{
			Group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID:          group3.GetID(),
					DisplayName: group3.DisplayName,
				},
				GroupTypes: []string{"security-groups"},
			},
		},
	}
	got := []*models.ListGroupsDeltaResponse{}
	for groupDeltas, err := range client.IterateGroupDeltas(ctx, endpoint, fakeDeltaStore) {
		require.NoError(t, err)

		got = append(got, groupDeltas)
	}
	require.Empty(t, cmp.Diff(expected, got, sortDeltas), "expected group delta response to match")

	fakeServer.DeleteGroups([]string{*group3.GetID()})
	fakeServer.DeleteUsers([]string{*alice.GetID()})
	// Add user alice and group3 as member
	carol := storage.Users[msgraphtest.CarolID]
	fakeServer.SetGroupOwners(*group1.GetID(), []*models.User{carol})
	expected = []*models.ListGroupsDeltaResponse{
		{
			Group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID:          group1.GetID(),
					DisplayName: group1.DisplayName,
				},
			},
			Members: []models.MembersDelta{
				{
					DirectoryObject: &models.DirectoryObject{
						ID: group3.GetID(),
					},
					Type: models.ODataGroup,
					Removed: &models.RemovedReason{
						Reason: to.Ptr("deleted"),
					},
				},
				{
					DirectoryObject: &models.DirectoryObject{
						ID: alice.GetID(),
					},
					Type: models.ODataUser,
					Removed: &models.RemovedReason{
						Reason: to.Ptr("deleted"),
					},
				},
			},
			Owners: []models.OwnersDelta{
				{
					User: &models.User{
						DirectoryObject: models.DirectoryObject{
							ID: carol.GetID(),
						},
					},
					Type: models.ODataUser,
				},
			},
		},
		{
			Group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: group3.GetID(),
				},
			},
			Removed: &models.RemovedReason{
				Reason: to.Ptr("deleted"),
			},
		},
	}
	got = []*models.ListGroupsDeltaResponse{}
	for groupDeltas, err := range client.IterateGroupDeltas(ctx, endpoint, fakeDeltaStore) {
		require.NoError(t, err)

		got = append(got, groupDeltas)
	}

	require.Empty(t, cmp.Diff(expected, got, sortDeltas), "expected group delta response to match")
}

func mustParseURL(t *testing.T, in string) *url.URL {
	t.Helper()
	url, err := url.Parse(in)
	require.NoError(t, err)
	require.Equal(t, "https", url.Scheme, "expected URL with https scheme")
	return url
}

func TestDeltaMethodsWithoutTop(t *testing.T) {
	ctx := t.Context()

	fakeServer := msgraphtest.NewServer()

	client, err := NewClient(Config{
		HTTPClient:    newHTTPClient(fakeServer.TLSServer),
		TokenProvider: &fakeTokenProvider{},
		RetryConfig:   &retryConfig,
		PageSize:      500, // pagesize added to ensure that the default $top query path is reached wihout the WithoutTop() option.
	})
	require.NoError(t, err)

	assertNoTop := func(t *testing.T, uri string) {
		t.Helper()
		parsed, err := url.ParseRequestURI(uri)
		if err != nil {
			t.Fatalf("failed to parse delta request URI %s: %v", uri, err)
		}
		if parsed.Query().Has("$top") {
			t.Fatalf("expected $top query to be absent in delta API requests, received: %s", uri)
		}
	}

	const userEndpoint = "users/delta"
	const groupEndpoint = "groups/delta"

	deltaStore := msgraphtest.NewFakeDeltaStore()

	t.Run("SetupLatestDelta", func(t *testing.T) {
		err = client.SetupLatestDelta(ctx, userEndpoint, deltaStore)
		require.NoError(t, err)
		assertNoTop(t, deltaStore.Get(userEndpoint))

		err = client.SetupLatestDelta(ctx, groupEndpoint, deltaStore)
		require.NoError(t, err)
		assertNoTop(t, deltaStore.Get(groupEndpoint))
	})

	t.Run("IterateUserDeltas", func(t *testing.T) {
		// This test should pass by default as long as SetupLatestDelta sets up
		// latest delta token without the $top query.
		for _, err = range client.IterateUserDeltas(ctx, userEndpoint, deltaStore) {
			require.NoError(t, err)
		}
		assertNoTop(t, deltaStore.Get(userEndpoint))
	})

	t.Run("IterateGroupDeltas", func(t *testing.T) {
		// This test should pass by default as long as SetupLatestDelta sets up
		// latest delta token without the $top query.
		for _, err = range client.IterateGroupDeltas(ctx, groupEndpoint, deltaStore) {
			require.NoError(t, err)
		}
		assertNoTop(t, deltaStore.Get(groupEndpoint))
	})
}

func TestValidateDeltaLink(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name           string
		baseURL        string
		deltaLink      string
		errorAssertion require.ErrorAssertionFunc
	}{
		{
			name:           "matching host",
			baseURL:        types.MSGraphDefaultEndpoint,
			deltaLink:      types.MSGraphDefaultEndpoint + "/v1.0/users/delta?$deltatoken=latest",
			errorAssertion: require.NoError,
		},
		{
			name:      "matching host with http sceheme",
			baseURL:   types.MSGraphDefaultEndpoint,
			deltaLink: "http://graph.microsoft.com/v1.0/users/delta?$deltatoken=latest",
			errorAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "scheme")
			},
		},
		{
			name:      "different host",
			baseURL:   types.MSGraphDefaultEndpoint,
			deltaLink: "https://cloudapp.azure.com" + "/v1.0/users/delta?$deltatoken=latest",
			errorAssertion: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "host mismatch")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeServer := msgraphtest.NewServer()
			t.Cleanup(func() { fakeServer.TLSServer.Close() })

			const endpoint = "users/delta"
			ds := msgraphtest.NewFakeDeltaStore()
			ds.Set(endpoint, tt.deltaLink)

			client, err := NewClient(Config{
				HTTPClient:    newHTTPClient(fakeServer.TLSServer),
				TokenProvider: &fakeTokenProvider{},
				RetryConfig:   &retryConfig,
				GraphEndpoint: tt.baseURL,
			})
			require.NoError(t, err)

			err = validateDeltaLink(client.baseURL, tt.deltaLink)
			tt.errorAssertion(t, err)

			for _, err := range client.iterateDelta(ctx, endpoint, ds) {
				tt.errorAssertion(t, err)
			}
		})
	}
}
