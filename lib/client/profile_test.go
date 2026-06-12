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

package client

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/services"
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
				WebProxyAddr:   "proxy1.example.com",
				Username:       "test-user",
				SiteName:       "root",
				Dir:            dir,
				SSHDialTimeout: 10 * time.Second,
			}, {
				WebProxyAddr:   "proxy2.example.com",
				Username:       "test-user",
				SiteName:       "root",
				Dir:            dir,
				SSHDialTimeout: 1 * time.Second,
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

func TestProfileStatusAccessInfo(t *testing.T) {
	allowedResourceIDs := []types.ResourceID{{
		ClusterName: "cluster",
		Kind:        types.KindNode,
		Name:        "uuid",
	}}
	traits := wrappers.Traits{
		"trait1": {"value1", "value2"},
		"trait2": {"value3", "value4"},
	}

	wantAccessInfo := &services.AccessInfo{
		Username:           "alice",
		Roles:              []string{"role1", "role2"},
		Traits:             traits,
		AllowedResourceIDs: allowedResourceIDs,
	}

	profileStatus := ProfileStatus{
		Username:           "alice",
		Roles:              []string{"role1", "role2"},
		Traits:             traits,
		AllowedResourceIDs: allowedResourceIDs,
	}

	require.Equal(t, wantAccessInfo, profileStatus.AccessInfo())
}

func Test_profileStatusFromKeyRing(t *testing.T) {
	auth := newTestAuthority(t)
	idx := KeyRingIndex{
		ProxyHost:   "proxy.example.com",
		ClusterName: "root",
		Username:    "test-user",
	}
	profile := &profile.Profile{
		WebProxyAddr: idx.ProxyHost + ":3080",
		SiteName:     idx.ClusterName,
		Username:     idx.Username,
	}
	keyRing := auth.makeSignedKeyRing(t, idx, false)
	profileStatus, err := profileStatusFromKeyRing(keyRing, profileOptions{
		ProfileName:       profile.Name(),
		WebProxyAddr:      profile.WebProxyAddr,
		ProfileDir:        "",
		Username:          profile.Username,
		SiteName:          profile.SiteName,
		KubeProxyAddr:     profile.KubeProxyAddr,
		IsVirtual:         true,
		TLSRoutingEnabled: true,
	})
	require.NoError(t, err)
	require.Equal(t, &ProfileStatus{
		Name:    "proxy.example.com",
		Cluster: "root",
		ProxyURL: url.URL{
			Scheme: "https",
			Host:   "proxy.example.com:3080",
		},
		Username: "test-user",
		Logins:   []string{"test-user", "root"},
		Extensions: []string{
			teleport.CertExtensionPermitPortForwarding,
			teleport.CertExtensionPermitPTY,
		},
		ValidUntil: time.Unix(auth.clock.Now().Add(20*time.Minute).Unix(), 0),
		IsVirtual:  true,
		GitHubIdentity: &GitHubIdentity{
			UserID:   "1234567",
			Username: "github-username",
		},
		CriticalOptions:   map[string]string{},
		TLSRoutingEnabled: true,
	}, profileStatus)
}
