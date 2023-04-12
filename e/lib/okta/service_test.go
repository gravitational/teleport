/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package okta

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestHandleConnection(t *testing.T) {
	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	svc, _ := newTestService(t, ap)

	// Create the test application.
	publicAddr := "https://public-addr"
	app := newApp(t, types.Metadata{
		Name: "test-app",
	}, types.AppSpecV3{
		PublicAddr: publicAddr,
		URI:        publicAddr,
	})
	svc.apps["test-app"] = app

	// Create the test role with access.
	role1, err := types.NewRole("role1", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				"*": utils.Strings{"*"},
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, ap.CreateRole(ctx, role1))

	// Create a test user with access.
	user1, err := types.NewUser("user1")
	require.NoError(t, err)
	user1.SetRoles([]string{role1.GetName()})
	require.NoError(t, ap.CreateUser(user1))

	// Create the test role without access.
	role2, err := types.NewRole("role2", types.RoleSpecV6{})
	require.NoError(t, err)
	require.NoError(t, ap.CreateRole(ctx, role2))

	// Create a test user without access.
	user2, err := types.NewUser("user2")
	require.NoError(t, err)
	user2.SetRoles([]string{role2.GetName()})
	require.NoError(t, ap.CreateUser(user2))

	// Create a test user that is not in the backend.
	user3, err := types.NewUser("user3")
	require.NoError(t, err)
	user3.SetRoles([]string{role2.GetName()})

	tests := []struct {
		name               string
		user               types.User
		publicAddr         string
		expectedStatusCode int
	}{
		{
			name:               "successful access",
			user:               user1,
			publicAddr:         publicAddr,
			expectedStatusCode: http.StatusFound,
		},
		{
			name:               "application does not exist",
			user:               user1,
			publicAddr:         "https://does-not-exist",
			expectedStatusCode: http.StatusNotFound,
		},
		{
			name:               "user does not have access",
			user:               user2,
			publicAddr:         publicAddr,
			expectedStatusCode: http.StatusNotFound,
		},
		{
			name:               "application does not exist and user does not have access",
			user:               user2,
			publicAddr:         publicAddr,
			expectedStatusCode: http.StatusNotFound,
		},
		{
			name:               "user does not exist",
			user:               user3,
			publicAddr:         publicAddr,
			expectedStatusCode: http.StatusNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			accessApp(t, svc, test.user, test.publicAddr, test.expectedStatusCode)
		})
	}
}

// accessApp will attempt to access thie application. The status code from the response from the client
// Get will be tested against the expected status code. If the expected status code is StatusFound, it
// will then be verified to ensure that the redirect is as expected.
func accessApp(t *testing.T, svc *Service, user types.User, publicAddr string, expectedStatusCode int) {
	listener, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, listener.Close())
	})

	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() {
		// Don't check the error here, as it may be closed already.
		conn.Close()
	})

	tlsConfig := generateTestTLSConfig(t, user.GetName(), user.GetRoles(), pkix.AttributeTypeAndValue{
		Type:  tlsca.AppPublicAddrASN1ExtensionOID,
		Value: publicAddr,
	})

	// Connect to our listener and pass that to the Okta service's HandleConnection.
	tlsConn := tls.Client(conn, tlsConfig)
	listenConn, err := listener.Accept()
	require.NoError(t, err)

	var handshakeWg sync.WaitGroup
	handshakeWg.Add(1)
	go func() {
		defer handshakeWg.Done()
		svc.HandleConnection(listenConn)
	}()

	client := http.Client{
		// Don't follow any redirects.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		// Only use the TLS connection.
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return tlsConn, nil
			},
		},
	}

	// Hit the service with given cert.
	url := "http://anything"
	resp, err := client.Get(url)
	require.NoError(t, err)

	require.Equal(t, expectedStatusCode, resp.StatusCode)

	if expectedStatusCode == http.StatusFound {
		// Capture the redirect contents.
		contents, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Create the expected redirect output.
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("GET", url, &bytes.Buffer{})
		http.Redirect(recorder, request, publicAddr, http.StatusFound)

		require.Equal(t, recorder.Body.String(), string(contents))
	}

	require.NoError(t, resp.Body.Close())

	handshakeWg.Wait()
}
