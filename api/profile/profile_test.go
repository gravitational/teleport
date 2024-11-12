/*
Copyright 2016-2021 Gravitational, Inc.

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

package profile_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/profile"
)

// TestProfileBasics verifies basic profile operations such as
// load/store and setting current.
func TestProfileBasics(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	p := &profile.Profile{
		WebProxyAddr:          "proxy:3088",
		SSHProxyAddr:          "proxy:3023",
		Username:              "testuser",
		DynamicForwardedPorts: []string{"localhost:8080"},
		Dir:                   dir,
		SiteName:              "example.com",
		AuthConnector:         "passwordless",
		MFAMode:               "auto",
	}

	// verify that profile name is proxy host component
	require.Equal(t, "proxy", p.Name())

	// save to a file:
	err := p.SaveToDir(dir, false)
	require.NoError(t, err)

	// verify that the resulting file exists and is of the form `<profile-dir>/<profile-name>.yaml`.
	_, err = os.Stat(filepath.Join(dir, p.Name()+".yaml"))
	require.NoError(t, err)

	// try to save to non-existent dir, should get an error
	err = p.SaveToDir("/bad/directory/", false)
	require.Error(t, err)

	// make sure current profile was not set
	_, err = profile.GetCurrentProfileName(dir)
	require.True(t, trace.IsNotFound(err))

	// save again, this time also making current
	err = p.SaveToDir(dir, true)
	require.NoError(t, err)

	// verify that current profile is set and matches this profile
	name, err := profile.GetCurrentProfileName(dir)
	require.NoError(t, err)
	require.Equal(t, p.Name(), name)

	// Update the dial timeout because when the profile is loaded, an
	// empty timeout is implicitly set to match the default value.
	p.SSHDialTimeout = defaults.DefaultIOTimeout

	// load and verify current profile
	clone, err := profile.FromDir(dir, "")
	require.NoError(t, err)
	require.Equal(t, *p, *clone)

	// load and verify directly
	clone, err = profile.FromDir(dir, p.Name())
	require.NoError(t, err)
	require.Equal(t, *p, *clone)
}

func TestAppPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	p := &profile.Profile{
		WebProxyAddr: "proxy:3088",
		SSHProxyAddr: "proxy:3023",
		Username:     "testuser",
		Dir:          dir,
		SiteName:     "example.com",
	}

	expectCertPath := filepath.Join(dir, "keys", "proxy", "testuser-app", "example.com", "banana.crt")
	require.Equal(t, expectCertPath, p.AppCertPath("banana"))
	expectKeyPath := filepath.Join(dir, "keys", "proxy", "testuser-app", "example.com", "banana.key")
	require.Equal(t, expectKeyPath, p.AppKeyPath("banana"))
}

func TestProfilePath(t *testing.T) {
	switch runtime.GOOS {
	case "darwin", "linux":
	default:
		t.Skip("this test only runs on Unix")
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	require.Equal(t, "/foo/bar", profile.FullProfilePath("/foo/bar"))
	require.Equal(t, filepath.Join(dir, ".tsh"), profile.FullProfilePath(""))
}

func TestRequireKubeLocalProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputProfile *profile.Profile
		checkResult  require.BoolAssertionFunc
	}{
		{
			name: "kube not enabled",
			inputProfile: &profile.Profile{
				WebProxyAddr:                  "example.com:443",
				TLSRoutingEnabled:             true,
				TLSRoutingConnUpgradeRequired: true,
			},
			checkResult: require.False,
		},
		{
			name: "ALPN connection upgrade not required",
			inputProfile: &profile.Profile{
				WebProxyAddr:      "example.com:443",
				KubeProxyAddr:     "example.com:443",
				TLSRoutingEnabled: true,
			},
			checkResult: require.False,
		},
		{
			name: "kube uses separate listener",
			inputProfile: &profile.Profile{
				WebProxyAddr:                  "example.com:443",
				KubeProxyAddr:                 "example.com:3026",
				TLSRoutingEnabled:             false,
				TLSRoutingConnUpgradeRequired: true,
			},
			checkResult: require.False,
		},
		{
			name: "local proxy required",
			inputProfile: &profile.Profile{
				WebProxyAddr:                  "example.com:443",
				KubeProxyAddr:                 "example.com:443",
				TLSRoutingEnabled:             true,
				TLSRoutingConnUpgradeRequired: true,
			},
			checkResult: require.True,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.checkResult(t, test.inputProfile.RequireKubeLocalProxy())
		})
	}
}
