// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package joinclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/clientversion"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// TestJoinFailsFastWhenClientTooOld ensures a confirmed too-old client gets
// [clientversion.ErrClientTooOld] back from [Join] itself. If the error were
// ever classified as a connection error anywhere in the chain, [Join] would
// instead fall back to the legacy join service, which has no version check
// and discards the original error.
func TestJoinFailsFastWhenClientTooOld(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/webapi/find" {
			http.NotFound(w, r)
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(webclient.PingResponse{
			MinClientVersion: teleport.MinClientSemVer().String(),
		}))
	}))
	t.Cleanup(srv.Close)

	tooOldVersion := semver.Version{Major: teleport.MinClientSemVer().Major - 1}

	_, err := Join(t.Context(), JoinParams{
		Token:       "token",
		ID:          state.IdentityID{Role: types.RoleInstance},
		ProxyServer: utils.NetAddr{AddrNetwork: "tcp", Addr: strings.TrimPrefix(srv.URL, "https://")},
		JoinMethod:  types.JoinMethodToken,
		Insecure:    true,
		Log:         logtest.NewLogger(),
		Testing:     JoinTestingParams{TeleportVersion: tooOldVersion.String()},
		// GetHostCredentials is only reached if the version check error is
		// misclassified and the legacy fallback engages. This stub makes that
		// regression fail legibly instead of panicking.
		GetHostCredentials: func(context.Context, string, bool, types.RegisterUsingTokenRequest) (*proto.Certs, error) {
			return nil, errors.New("host credentials unavailable")
		},
	})
	require.ErrorIs(t, err, clientversion.ErrClientTooOld)
}
