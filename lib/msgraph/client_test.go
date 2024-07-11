package msgraph

import (
	"context"
	"encoding/json"
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
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		_, err := strconv.Atoi(r.URL.Query().Get("$top"))
		if err != nil {
			assert.Fail(t, "expected to get $top parameter")
		}
		w.Write([]byte(`{"value": []}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close() })

	uri, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := &client{
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
	var sourceUsers []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(usersPayload), &sourceUsers))
	mux := http.NewServeMux()
	mux.Handle("GET /users", paginatedHandler(t, sourceUsers))

	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close() })

	uri, err := url.Parse(srv.URL)
	require.NoError(t, err)
	client := &client{
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

	require.Equal(t, "bob@example.com", *users[1].Mail)
	require.Equal(t, "bob@example.com", *users[1].UserPrincipalName)

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
		assert.Equal(f.t, body, f.expectedBody)
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
		client := &client{
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
		require.Eventually(t, func() bool { return handler.timesCalled.Load() == 1 }, time.Second, time.Second/100)

		// Fail for the second time
		clock.Advance(time.Duration(handler.retryAfter) * time.Second)
		require.Eventually(t, func() bool { return handler.timesCalled.Load() == 2 }, time.Second, time.Second/100)

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
			expectedBody:   []byte(`{"name":"foo"}`),
			successPayload: objPayload,
		}
		mux := http.NewServeMux()
		mux.Handle(route, handler)

		srv := httptest.NewServer(mux)
		t.Cleanup(func() { srv.Close() })

		uri, err := url.Parse(srv.URL)
		require.NoError(t, err)
		client := &client{
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
		require.Eventually(t, func() bool { return handler.timesCalled.Load() == 1 }, time.Second, time.Second/100)

		// Fail for the second time
		clock.Advance(time.Second)
		require.Eventually(t, func() bool { return handler.timesCalled.Load() == 2 }, time.Second, time.Second/100)

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
			expectedBody: []byte(`{"name":"foo"}`),
		}
		mux := http.NewServeMux()
		mux.Handle(route, handler)

		srv := httptest.NewServer(mux)
		t.Cleanup(func() { srv.Close() })

		uri, err := url.Parse(srv.URL)
		require.NoError(t, err)
		client := &client{
			httpClient:    &http.Client{},
			tokenProvider: &fakeTokenProvider{},
			clock:         clock,
			baseURL:       uri,
		}

		_, err = client.CreateFederatedIdentityCredential(context.Background(), appID, fic)
		require.Error(t, err)
	})
}
