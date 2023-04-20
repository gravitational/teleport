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

package common

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
)

func TestLocks(t *testing.T) {
	dynAddr := newDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.webAddr,
			TunAddr: dynAddr.tunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.authAddr,
			},
		},
	}

	timeNow := time.Now().UTC()
	fakeClock := clockwork.NewFakeClockAt(timeNow)
	makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.descriptors), withFakeClock(fakeClock))

	t.Run("create", func(t *testing.T) {
		err := runLockCommand(t, fileConfig, []string{"--user=bad@actor", "--message=Come see me"})
		require.NoError(t, err)

		var out []*types.LockV2
		buf, err := runResourceCommand(t, fileConfig, []string{"get", types.KindLock, "--format=json"})
		require.NoError(t, err)
		mustDecodeJSON(t, buf, &out)

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
