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

	fakeDevices := &fakeDevicesClient{}
	wPack := newWebPack(
		t,
		1, /* numProxies */
		withDevicesClientOverride(fakeDevices),
	)

	proxy := wPack.proxies[0]
	aPack := proxy.authPack(t, "llama", nil /* roles */)
	webClient := aPack.clt

	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		query := make(url.Values)
		query.Set("id", "my-token-id")
		query.Set("token", "my-token-token")

		// Detect client redirects.
		var redirected bool
		httpClient := webClient.HTTPClient()
		httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			// Ignore any subsequent redirects that may happen.
			if redirected {
				return nil
			}

			redirected = true
			if !assert.Len(t, via, 1, "CheckRedirect param via has an unexpected length") {
				return nil
			}
			src := via[0]

			// Host didn't change, ie redirect is within the same Proxy.
			assert.Equal(t, src.URL.Host, req.URL.Host, "CheckRedirect Host mismatch")
			// Redirect target is as expected.
			assert.Equal(t, "/web", req.URL.Path, "CheckRedirect dest Path mismatch")
			// Redirect source is as expected.
			assert.Regexp(t, "/webapi/devices/webconfirm$", src.URL.Path, "CheckRedirect src Path mismatch")

			return nil
		}

		req, err := http.NewRequestWithContext(ctx, "GET", webClient.Endpoint("webapi", "devices", "webconfirm"), nil /* body */)
		require.NoError(t, err, "NewRequestWithContext failed")
		req.URL.RawQuery = query.Encode()

		// Request using the httpClient, this shows we don't need the bearer token
		// logic from webclient.
		resp, err := httpClient.Do(req)
		require.NoError(t, err, "GET /webapi/devices/webconfirm failed")
		// Always drain and close the body.
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		// Verify redirect and response status.
		assert.True(t, redirected, "GET /webapi/devices/webconfirm didn't cause a redirect")
		assert.Equal(t, 200, resp.StatusCode, "GET /webapi/devices/webconfirm code mismatch")

		// Verify RPC call.
		got := fakeDevices.resetConfirmRequests()
		want := []*devicepb.ConfirmDeviceWebAuthenticationRequest{
			{
				ConfirmationToken: &devicepb.DeviceConfirmationToken{
					Id:    "my-token-id",
					Token: "my-token-token",
				},
			},
		}
		// Copy WebSessionID from got to want.
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
