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

package web

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

func TestHandler_DeviceWebConfirm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		redirectURI        string
		expectedRedirectTo string
	}{
		{
			name:               "no redirect_uri",
			redirectURI:        "",
			expectedRedirectTo: "/web",
		},
		{
			name:               "with redirect_uri",
			redirectURI:        "https://example.com/web/custom/path",
			expectedRedirectTo: "/web/custom/path",
		},
		{
			name:               "with app access redirect_uri",
			redirectURI:        "https://example.com/web/launch/myapp.example.com",
			expectedRedirectTo: "/web/launch/myapp.example.com",
		},
		{
			name:               "with invalid redirect_uri",
			redirectURI:        "://invalid",
			expectedRedirectTo: "/web",
		},
		{
			name:               "with external redirect_uri",
			redirectURI:        "https://example.com/path",
			expectedRedirectTo: "/web/path",
		},
		{
			name:               "with empty path redirect_uri",
			redirectURI:        "https://example.com",
			expectedRedirectTo: "/web",
		},
	}

	fakeDevices := &fakeDevicesClient{}
	wPack := newWebPack(
		t,
		1, /* numProxies */
		withDevicesClientOverride(fakeDevices),
	)
	proxy := wPack.proxies[0]
	aPack := proxy.authPack(t, "llama", nil /* roles */)
	webClient := aPack.clt

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			query := make(url.Values)
			query.Set("id", "my-token-id")
			query.Set("token", "my-token-token")
			if test.redirectURI != "" {
				query.Set("redirect_uri", test.redirectURI)
			}

			var redirected bool
			var actualRedirectTo string
			httpClient := webClient.HTTPClient()
			httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				if !redirected {
					redirected = true
					actualRedirectTo = req.URL.Path
				}
				return nil
			}

			req, err := http.NewRequestWithContext(ctx, "GET", webClient.Endpoint("webapi", "devices", "webconfirm"), nil)
			require.NoError(t, err, "NewRequestWithContext failed")
			req.URL.RawQuery = query.Encode()

			resp, err := httpClient.Do(req)
			require.NoError(t, err, "GET /webapi/devices/webconfirm failed")
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			assert.True(t, redirected, "GET /webapi/devices/webconfirm didn't cause a redirect")
			assert.Equal(t, http.StatusOK, resp.StatusCode, "GET /webapi/devices/webconfirm code mismatch")
			assert.Equal(t, test.expectedRedirectTo, actualRedirectTo, "Redirect destination mismatch")

			got := fakeDevices.resetConfirmRequests()
			want := []*devicepb.ConfirmDeviceWebAuthenticationRequest{
				{
					ConfirmationToken: &devicepb.DeviceConfirmationToken{
						Id:    "my-token-id",
						Token: "my-token-token",
					},
				},
			}

			if len(got) > 0 {
				webSessionID := got[0].CurrentWebSessionId
				assert.NotEmpty(t, webSessionID, "ConfirmDeviceWebAuthentication called with empty WebSessionID")
				want[0].CurrentWebSessionId = webSessionID
			}

			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("ConfirmDeviceWebAuthentication requests mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

type fakeDevicesClient struct {
	devicepb.DeviceTrustServiceClient // used only to "implement" the interface, typically left nil

	mu                    sync.Mutex
	confirmDeviceRequests []*devicepb.ConfirmDeviceWebAuthenticationRequest
}

func (f *fakeDevicesClient) ConfirmDeviceWebAuthentication(ctx context.Context, req *devicepb.ConfirmDeviceWebAuthenticationRequest, opts ...grpc.CallOption) (*devicepb.ConfirmDeviceWebAuthenticationResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Save request for later inspection.
	f.confirmDeviceRequests = append(f.confirmDeviceRequests, req)

	// Successful response.
	return &devicepb.ConfirmDeviceWebAuthenticationResponse{}, nil
}

func (f *fakeDevicesClient) resetConfirmRequests() []*devicepb.ConfirmDeviceWebAuthenticationRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	reqs := f.confirmDeviceRequests
	f.confirmDeviceRequests = nil

	return reqs
}

func TestHandler_GetRedirectURL(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name           string
		redirectURI    string
		expectedURL    string
		expectedErrMsg string
	}{
		{
			name:        "no redirect_uri",
			redirectURI: "",
			expectedURL: "/web",
		},
		{
			name:        "with redirect_uri",
			redirectURI: "https://example.com/web/custom/path",
			expectedURL: "/web/custom/path",
		},
		{
			name:        "with app access redirect_uri",
			redirectURI: "https://example.com/web/launch/myapp.example.com",
			expectedURL: "/web/launch/myapp.example.com",
		},
		{
			name:           "with invalid redirect_uri",
			redirectURI:    "://invalid",
			expectedURL:    "/web",
			expectedErrMsg: "parse \"://invalid\": missing protocol scheme",
		},
		{
			name:        "with external redirect_uri",
			redirectURI: "https://example.com/path",
			expectedURL: "/web/path",
		},
		{
			name:        "with empty path redirect_uri",
			redirectURI: "https://example.com",
			expectedURL: "/web",
		},
		{
			name:        "with relative path",
			redirectURI: "/custom/path",
			expectedURL: "/web/custom/path",
		},
		{
			name:        "with web prefix already",
			redirectURI: "/web/existing/path",
			expectedURL: "/web/existing/path",
		},
		{
			name:           "with completely invalid URI",
			redirectURI:    "not a URI at all",
			expectedURL:    "/web",
			expectedErrMsg: "parse \"not a URI at all\": invalid URI for request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := h.getRedirectURL(tt.redirectURI)
			assert.Equal(t, tt.expectedURL, result)

			if tt.expectedErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
