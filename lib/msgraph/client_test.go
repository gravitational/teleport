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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

// Always sleep for a second for predictability
var retryConfig = retryutils.RetryV2Config{
	First:  time.Second,
	Max:    time.Second,
	Driver: retryutils.NewLinearDriver(time.Second),
}

type fakeTokenProvider struct{}

func (t *fakeTokenProvider) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token: "foo",
	}, nil
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
		nextLink.Scheme = "http"
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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		_, err := strconv.Atoi(r.URL.Query().Get("$top"))
		assert.NoError(t, err, "expected to get $top parameter")
		w.Write([]byte(`{"value": []}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close() })

	uri, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := &Client{
		httpClient:    &http.Client{},
		tokenProvider: &fakeTokenProvider{},
		retryConfig:   retryConfig,
		baseURL:       uri,
		pageSize:      defaultPageSize,
	}
	err = client.IterateUsers(context.Background(), func(*User) bool {
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
	mux.Handle("GET /users", paginatedHandler(t, sourceUsers))

	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close() })

	uri, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := &Client{
		httpClient:    &http.Client{},
		tokenProvider: &fakeTokenProvider{},
		retryConfig:   retryConfig,
		baseURL:       uri,
		pageSize:      2, // smaller page size so we actually fetch multiple pages with our small test payload
	}

	var users []*User
	err = client.IterateUsers(context.Background(), func(u *User) bool {
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
	route := "POST /applications/" + appID + "/federatedIdentityCredentials"
	name := "foo"
	fic := &FederatedIdentityCredential{Name: &name}
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

		srv := httptest.NewServer(mux)
		t.Cleanup(func() { srv.Close() })

		uri, err := url.Parse(srv.URL)
		require.NoError(t, err)
		client := &Client{
			httpClient:    &http.Client{},
			tokenProvider: &fakeTokenProvider{},
			clock:         clock,
			retryConfig:   retryConfig,
			baseURL:       uri,
		}

		ret := make(chan error, 1)
		go func() {
			out, err := client.CreateFederatedIdentityCredential(context.Background(), appID, fic)
			assert.Equal(t, fic, out)
			ret <- err
		}()

		// Fail for the first time
		clock.BlockUntil(1)
		require.EqualValues(t, 1, handler.timesCalled.Load())

		// Fail for the second time
		clock.Advance(time.Duration(handler.retryAfter) * time.Second)
		clock.BlockUntil(1)
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

		srv := httptest.NewServer(mux)
		t.Cleanup(func() { srv.Close() })

		uri, err := url.Parse(srv.URL)
		require.NoError(t, err)
		client := &Client{
			httpClient:    &http.Client{},
			tokenProvider: &fakeTokenProvider{},
			clock:         clock,
			retryConfig:   retryConfig,
			baseURL:       uri,
		}

		ret := make(chan error, 1)
		go func() {
			out, err := client.CreateFederatedIdentityCredential(context.Background(), appID, fic)
			assert.Equal(t, fic, out)
			ret <- err
		}()

		// Fail for the first time
		clock.BlockUntil(1)
		require.EqualValues(t, 1, handler.timesCalled.Load())

		// Fail for the second time
		clock.Advance(time.Second)
		clock.BlockUntil(1)
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

		srv := httptest.NewServer(mux)
		t.Cleanup(func() { srv.Close() })

		uri, err := url.Parse(srv.URL)
		require.NoError(t, err)
		client := &Client{
			httpClient:    &http.Client{},
			tokenProvider: &fakeTokenProvider{},
			clock:         clock,
			baseURL:       uri,
		}

		_, err = client.CreateFederatedIdentityCredential(context.Background(), appID, fic)
		require.Error(t, err)
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
	mux.Handle("GET /groups/"+groupID+"/members", paginatedHandler(t, membersJSON))

	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close() })

	uri, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := &Client{
		httpClient:    &http.Client{},
		tokenProvider: &fakeTokenProvider{},
		retryConfig:   retryConfig,
		baseURL:       uri,
		pageSize:      2, // smaller page size so we actually fetch multiple pages with our small test payload
	}

	var members []GroupMember
	err = client.IterateGroupMembers(context.Background(), groupID, func(u GroupMember) bool {
		members = append(members, u)
		return true
	})

	require.NoError(t, err)
	require.Len(t, members, 2)
	{
		require.IsType(t, &User{}, members[0])
		user := members[0].(*User)
		require.Equal(t, "9f615773-8219-4a5e-9eb1-8e701324c683", *user.ID)
		require.Equal(t, "alice@example.com", *user.Mail)
	}
	{
		require.IsType(t, &Group{}, members[1])
		group := members[1].(*Group)
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
	mux.Handle(fmt.Sprintf("GET /applications(appId='%s')", appID),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(getApplicationPayload))
		}))

	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close() })

	uri, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := &Client{
		httpClient:    &http.Client{},
		tokenProvider: &fakeTokenProvider{},
		retryConfig:   retryConfig,
		baseURL:       uri,
		pageSize:      2, // smaller page size so we actually fetch multiple pages with our small test payload
	}

	app, err := client.GetApplication(context.Background(), appID)
	require.NoError(t, err)
	require.Equal(t, "aeee7e9f-57ad-4ea6-a236-cd10b2dbc0b4", *app.ID)

	expectation := &Application{
		AppID: toPtr("d2a39a2a-1636-457f-82f9-c2d76527e20e"),
		DirectoryObject: DirectoryObject{
			DisplayName: toPtr("test SAML App"),
			ID:          toPtr("aeee7e9f-57ad-4ea6-a236-cd10b2dbc0b4"),
		},
		GroupMembershipClaims: toPtr("SecurityGroup"),
		IdentifierURIs:        &[]string{"goteleport.com"},
		OptionalClaims: &OptionalClaims{
			AccessToken: []OptionalClaim{},
			IDToken:     []OptionalClaim{},
			SAML2Token: []OptionalClaim{
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
