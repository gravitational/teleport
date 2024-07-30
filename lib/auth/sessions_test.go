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

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestServer_CreateWebSessionFromReq_deviceWebToken(t *testing.T) {
	t.Parallel()

	testAuthServer, err := NewTestAuthServer(TestAuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err, "NewTestAuthServer failed")
	t.Cleanup(func() {
		assert.NoError(t, testAuthServer.Close(), "testAuthServer.Close() errored")
	})

	authServer := testAuthServer.AuthServer
	ctx := context.Background()

	// Wire a fake CreateDeviceWebTokenFunc to authServer.
	fakeWebToken := &devicepb.DeviceWebToken{
		Id:    "423f10ed-c3c1-4de7-99dc-3bc5b9ab7fd5",
		Token: "409d21e4-9563-497f-9393-1209f9e4289c",
	}
	wantToken := &types.DeviceWebToken{
		Id:    fakeWebToken.Id,
		Token: fakeWebToken.Token,
	}
	authServer.SetCreateDeviceWebTokenFunc(func(ctx context.Context, dwt *devicepb.DeviceWebToken) (*devicepb.DeviceWebToken, error) {
		return fakeWebToken, nil
	})

	const userLlama = "llama"
	user, _, err := CreateUserAndRole(authServer, userLlama, []string{userLlama} /* logins */, nil /* allowRules */)
	require.NoError(t, err, "CreateUserAndRole failed")

	// Arbitrary, real-looking values.
	const loginIP = "40.89.244.232"
	const loginUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36"

	t.Run("ok", func(t *testing.T) {
		session, err := authServer.CreateWebSessionFromReq(ctx, NewWebSessionRequest{
			User:                 userLlama,
			LoginIP:              loginIP,
			LoginUserAgent:       loginUserAgent,
			Roles:                user.GetRoles(),
			Traits:               user.GetTraits(),
			SessionTTL:           1 * time.Minute,
			LoginTime:            time.Now(),
			CreateDeviceWebToken: true,
		})
		require.NoError(t, err, "CreateWebSessionFromReq failed")

		gotToken := session.GetDeviceWebToken()
		if diff := cmp.Diff(wantToken, gotToken); diff != "" {
			t.Errorf("CreateWebSessionFromReq DeviceWebToken mismatch (-want +got)\n%s", diff)
		}
	})
}
