/*
Copyright 2016-2022 Gravitational, Inc.

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

package client

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/profile"
)

func newTestFSProfileStore(t *testing.T) *FSProfileStore {
	fsProfileStore := NewFSProfileStore(t.TempDir())
	return fsProfileStore
}

func testEachProfileStore(t *testing.T, testFunc func(t *testing.T, profileStore ProfileStore)) {
	t.Run("FS", func(t *testing.T) {
		testFunc(t, newTestFSProfileStore(t))
	})

	t.Run("Mem", func(t *testing.T) {
		testFunc(t, NewMemProfileStore())
	})
}

func TestProfileStore(t *testing.T) {
	t.Parallel()

	testEachProfileStore(t, func(t *testing.T, profileStore ProfileStore) {
		var dir string
		if fsProfileStore, ok := profileStore.(*FSProfileStore); ok {
			dir = fsProfileStore.Dir
		}
		profiles := []*profile.Profile{
			{
				WebProxyAddr: "proxy1.example.com",
				Username:     "test-user",
				SiteName:     "root",
				Dir:          dir,
			}, {
				WebProxyAddr: "proxy2.example.com",
				Username:     "test-user",
				SiteName:     "root",
				Dir:          dir,
			},
		}

		err := profileStore.SaveProfile(profiles[0], true)
		require.NoError(t, err)
		err = profileStore.SaveProfile(profiles[1], false)
		require.NoError(t, err)

		current, err := profileStore.CurrentProfile()
		require.NoError(t, err)
		require.Equal(t, "proxy1.example.com", current)

		listProfiles, err := profileStore.ListProfiles()
		require.NoError(t, err)
		require.Len(t, listProfiles, 2)
		require.ElementsMatch(t, []string{"proxy1.example.com", "proxy2.example.com"}, listProfiles)

		retProfiles := make([]*profile.Profile, 2)
		for i, profileName := range listProfiles {
			profile, err := profileStore.GetProfile(profileName)
			require.NoError(t, err)
			retProfiles[i] = profile
		}
		require.ElementsMatch(t, profiles, retProfiles)
	})
}

func TestProfileNameFromProxyAddress(t *testing.T) {
	t.Parallel()

	store := NewMemProfileStore()
	require.NoError(t, store.SaveProfile(&profile.Profile{
		WebProxyAddr: "proxy1.example.com:443",
		Username:     "test-user",
		SiteName:     "root",
	}, true))

	t.Run("current profile", func(t *testing.T) {
		profileName, err := ProfileNameFromProxyAddress(store, "")
		require.NoError(t, err)
		require.Equal(t, "proxy1.example.com", profileName)
	})
	t.Run("proxy host", func(t *testing.T) {
		profileName, err := ProfileNameFromProxyAddress(store, "proxy2.example.com:443")
		require.NoError(t, err)
		require.Equal(t, "proxy2.example.com", profileName)
	})
	t.Run("invalid proxy address", func(t *testing.T) {
		_, err := ProfileNameFromProxyAddress(store, ":443")
		require.Error(t, err)
	})
}
