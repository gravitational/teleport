/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestLocks(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.WebAddr,
			TunAddr: dynAddr.TunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}

	timeNow := time.Now().UTC()
	fakeClock := clockwork.NewFakeClockAt(timeNow)
	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors), withFakeClock(fakeClock))
	clt := testenv.MakeDefaultAuthClient(t, process)

	t.Run("create", func(t *testing.T) {
		err := runLockCommand(t, clt, []string{"--user=bad@actor", "--message=Come see me"})
		require.NoError(t, err)

		buf, err := runResourceCommand(t, clt, []string{"get", types.KindLock, "--format=json"})
		require.NoError(t, err)
		out := mustDecodeJSON[[]*types.LockV2](t, buf)

		expected, err := types.NewLock("test-lock", types.LockSpecV2{
			Target: types.LockTarget{
				User: "bad@actor",
			},
			Message: "Come see me",
		})
		require.NoError(t, err)
		expected.SetCreatedBy(string(types.RoleAdmin))

		require.NoError(t, err)
		expected.SetCreatedAt(timeNow)

		require.Empty(t, cmp.Diff([]*types.LockV2{expected.(*types.LockV2)}, out,
			cmpopts.IgnoreFields(types.LockV2{}, "Metadata")))
	})
}
