/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package network

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/azure/client"
)

// TestInterfacesClientLive exercises the network interfaces client against a
// real Azure subscription. It is skipped unless TELEPORT_TEST_AZURE is set.
//
// Prerequisites:
//   - Authenticate so DefaultAzureCredential can find a credential, e.g. run
//     `az login`, or set AZURE_TENANT_ID / AZURE_CLIENT_ID / AZURE_CLIENT_SECRET.
//     The identity needs Microsoft.Network/networkInterfaces/read on the NIC.
//   - Set TELEPORT_TEST_AZURE_NIC_ID to a NIC resource ID, e.g.
//     /subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Network/networkInterfaces/<name>
//
// Run with:
//
//	TELEPORT_TEST_AZURE=1 \
//	TELEPORT_TEST_AZURE_NIC_ID=/subscriptions/.../networkInterfaces/my-nic \
//	go test ./lib/cloud/azure/network/ -run TestInterfacesClientLive -v
func TestInterfacesClientLive(t *testing.T) {
	if os.Getenv("TELEPORT_TEST_AZURE") == "" {
		t.Skip("Set TELEPORT_TEST_AZURE to run this test against a real Azure subscription.")
	}
	nicID := os.Getenv("TELEPORT_TEST_AZURE_NIC_ID")
	if nicID == "" {
		t.Skip("Set TELEPORT_TEST_AZURE_NIC_ID to the resource ID of a real Azure network interface.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	require.NoError(t, err, "failed to build a default Azure credential; have you run `az login`?")

	queryClient, err := client.NewClient(cred, client.WithRetryOnRateLimitErrors())
	require.NoError(t, err)

	nicClient := NewInterfacesClient(queryClient)

	nic, err := nicClient.Get(ctx, nicID)
	require.NoError(t, err)
	require.NotNil(t, nic)

	t.Logf("NIC %q: private IP=%q", nic.Name, nic.PrivateIP)

	require.Equal(t, nicID, nic.ID)
	require.NotEmpty(t, nic.PrivateIP, "expected the NIC to have a private IP")
}

const testNICID = "/subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.Network/networkInterfaces/my-nic"
const token = "fake-token"

// fakeDoer is an HTTP doer that returns a canned response and records the
// request it received, so tests can run without contacting Azure.
type fakeDoer struct {
	resp  *http.Response
	err   error
	calls int
	last  *http.Request
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.calls++
	f.last = req
	if f.err != nil {
		return nil, f.err
	}
	f.resp.Request = req
	return f.resp, nil
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func newTestClient(t *testing.T, doer *fakeDoer) InterfacesClient {
	t.Helper()
	cred := azure.NewStaticCredential(azcore.AccessToken{Token: token})
	queryClient, err := client.NewClient(cred, client.WithHTTPClient(doer))
	require.NoError(t, err)
	return NewInterfacesClient(queryClient)
}

func TestInterfacesClientGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		body          string
		wantName      string
		wantPrivateIP string
	}{
		{
			name:          "single primary ip config",
			body:          `{"name":"my-nic","properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"10.0.0.4","primary":true}}]}}`,
			wantName:      "my-nic",
			wantPrivateIP: "10.0.0.4",
		},
		{
			name:          "multiple ip configs, primary not first",
			body:          `{"name":"my-nic","properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"10.0.0.5"}},{"properties":{"privateIPAddress":"10.0.0.6","primary":true}}]}}`,
			wantName:      "my-nic",
			wantPrivateIP: "10.0.0.6",
		},
		{
			name:          "no primary flagged falls back to the only ip",
			body:          `{"name":"my-nic","properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"10.0.0.7"}}]}}`,
			wantName:      "my-nic",
			wantPrivateIP: "10.0.0.7",
		},
		{
			name:          "no primary flagged falls back to the first ip",
			body:          `{"name":"my-nic","properties":{"ipConfigurations":[{"properties":{"privateIPAddress":"10.0.0.9"}},{"properties":{"privateIPAddress":"10.0.0.10"}}]}}`,
			wantName:      "my-nic",
			wantPrivateIP: "10.0.0.9",
		},
		{
			name:          "empty ip config is skipped",
			body:          `{"name":"my-nic","properties":{"ipConfigurations":[{"properties":{"privateIPAddress":""}},{"properties":{"privateIPAddress":"10.0.0.8"}}]}}`,
			wantName:      "my-nic",
			wantPrivateIP: "10.0.0.8",
		},
		{
			name:          "no properties",
			body:          `{"name":"my-nic"}`,
			wantName:      "my-nic",
			wantPrivateIP: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			doer := &fakeDoer{resp: jsonResponse(http.StatusOK, test.body)}
			nic, err := newTestClient(t, doer).Get(context.Background(), testNICID)
			require.NoError(t, err)
			require.Equal(t, testNICID, nic.ID)
			require.Equal(t, test.wantName, nic.Name)
			require.Equal(t, test.wantPrivateIP, nic.PrivateIP)
		})
	}
}

func TestInterfacesClientGetRequest(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{resp: jsonResponse(http.StatusOK, `{"name":"my-nic"}`)}
	_, err := newTestClient(t, doer).Get(context.Background(), testNICID)
	require.NoError(t, err)

	require.Equal(t, 1, doer.calls)
	req := doer.last
	require.NotNil(t, req)
	require.Equal(t, http.MethodGet, req.Method)
	require.Equal(t, "management.azure.com", req.URL.Host)
	require.Equal(t, testNICID, req.URL.Path)
	require.Equal(t, interfaceAPIVersion, req.URL.Query().Get("api-version"))
	require.Equal(t, "Bearer "+token, req.Header.Get("Authorization"))
}

func TestInterfacesClientGetErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		status    int
		assertErr func(error) bool
	}{
		{"not found", http.StatusNotFound, trace.IsNotFound},
		{"access denied", http.StatusForbidden, trace.IsAccessDenied},
		{"server error", http.StatusInternalServerError, func(err error) bool { return err != nil }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			doer := &fakeDoer{resp: jsonResponse(test.status, `{"error":{"code":"fail","message":"bye"}}`)}
			_, err := newTestClient(t, doer).Get(context.Background(), testNICID)
			require.Error(t, err)
			require.True(t, test.assertErr(err), "unexpected error type: %v", err)
		})
	}
}

func TestInterfacesClientGetInvalidResourceID(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{resp: jsonResponse(http.StatusOK, `{}`)}
	_, err := newTestClient(t, doer).Get(context.Background(), "not-a-valid-resource-id")
	require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
	require.Zero(t, doer.calls, "transport must not be called for an invalid resource ID")
}
