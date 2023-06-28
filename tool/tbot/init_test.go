/*
Copyright 2022 Gravitational, Inc.

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

package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

// usernamesToTry contains a list of usernames we can use as ACL targets in
// testing.
var usernamesToTry = []string{"nobody", "ci", "root"}

// filterUsers returns the input list of usernames except for those in the
// exclude list.
func filterUsers(usernames, exclude []string) []string {
	ret := []string{}

	for _, username := range usernames {
		if !slices.Contains(exclude, username) {
			ret = append(ret, username)
		}
	}

	return ret
}

// findUser attempts to find a usable user on the local system from the given
// list of usernames and returns the first match found.
func findUser(usernamesToTry, usernamesToExclude []string) (*user.User, error) {
	filtered := filterUsers(usernamesToTry, usernamesToExclude)
	for _, username := range filtered {
		u, err := user.Lookup(username)
		if err == nil {
			return u, nil
		}
	}

	return nil, trace.NotFound("No users found matching %+v (excluding %+v)", usernamesToTry, usernamesToExclude)
}

// getACLOptions returns sane ACLOptions for this platform.
func getACLOptions() (*botfs.ACLOptions, error) {
	if runtime.GOOS != constants.LinuxOS {
		return nil, trace.NotImplemented("Unsupported platform")
	}

	user, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	exclude := []string{user.Name}

	// Find a set of users we can test against.
	readerUser, err := findUser(usernamesToTry, exclude)
	if trace.IsNotFound(err) {
		return nil, trace.NotFound("Not enough usable users for testing ACLs")
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	exclude = append(exclude, readerUser.Name)
	botUser, err := findUser(usernamesToTry, exclude)
	if trace.IsNotFound(err) {
		return nil, trace.NotFound("Not enough suitable users found for testing ACLs.")
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	return &botfs.ACLOptions{
		ReaderUser: readerUser,
		BotUser:    botUser,
	}, nil
}

// testConfigFromCLI creates a BotConfig from the given CLI config.
func testConfigFromCLI(t *testing.T, cf *config.CLIConf) *config.BotConfig {
	cfg, err := config.FromCLIConf(cf)
	require.NoError(t, err)

	return cfg
}

// testConfigFromString parses a YAML config file from a string.
func testConfigFromString(t *testing.T, yaml string) *config.BotConfig {
	cfg, err := config.ReadConfig(strings.NewReader(yaml))
	require.NoError(t, err)

	return cfg
}

// validateFileDestinations ensures all files in a destination exist on disk as
// expected, and returns the destination.
func validateFileDestination(t *testing.T, output config.Output) *config.DestinationDirectory {
	destImpl := output.GetDestination()

	destDir, ok := destImpl.(*config.DestinationDirectory)
	require.True(t, ok)

	for _, art := range identity.GetArtifacts() {
		if !art.Matches(identity.DestinationKinds()...) {
			continue
		}

		require.FileExists(t, filepath.Join(destDir.Path, art.Key))
	}

	return destDir
}

// TestInit ensures defaults work regardless of host platform. With no bot user
// specified, this never tries to use ACLs.
func TestInit(t *testing.T) {
	dir := t.TempDir()
	cf := &config.CLIConf{
		AuthServer:     "example.com",
		DestinationDir: dir,
	}
	cfg := testConfigFromCLI(t, cf)

	// Run init.
	require.NoError(t, onInit(cfg, cf))

	// Make sure everything was created.
	_ = validateFileDestination(t, cfg.Outputs[0])
}

// TestInitMaybeACLs tests defaults with ACLs possibly enabled, by supplying
// bot and reader users.
func TestInitMaybeACLs(t *testing.T) {
	opts, err := getACLOptions()
	if trace.IsNotImplemented(err) {
		t.Skipf("%+v", err)
	} else if trace.IsNotFound(err) {
		t.Skipf("%+v", err)
	}
	require.NoError(t, err)

	hasACLSupport, err := botfs.HasACLSupport()
	require.NoError(t, err)

	currentUser, err := user.Current()
	require.NoError(t, err)

	currentGroup, err := user.LookupGroupId(currentUser.Gid)
	require.NoError(t, err)

	// Determine if we expect init to use ACLs.
	expectACLs := false
	if hasACLSupport {
		if err := testACL(t.TempDir(), currentUser, opts); err == nil {
			expectACLs = true
		}
	}

	// Note: we'll use the current user as owner as that's the only way to
	// guarantee ACL write access.
	dir := t.TempDir()
	cf := &config.CLIConf{
		AuthServer:     "example.com",
		DestinationDir: dir,
		BotUser:        opts.BotUser.Username,
		ReaderUser:     opts.ReaderUser.Username,

		// This isn't a default, but unfortunately we need to specify a
		// non-nobody owner for CI purposes.
		Owner: fmt.Sprintf("%s:%s", currentUser.Username, currentGroup.Name),
	}
	cfg := testConfigFromCLI(t, cf)

	// Run init.
	require.NoError(t, onInit(cfg, cf))

	// Make sure everything was created.
	destDir := validateFileDestination(t, cfg.Outputs[0])

	// If we expect ACLs, verify them.
	if expectACLs {
		require.NoError(t, destDir.Verify(identity.ListKeys(identity.DestinationKinds()...)))
	} else {
		t.Logf("Skipping ACL check on %q as they should not be supported.", dir)
	}
}

// testInitSymlinksTemplate is a config template with a configurable symlinks
// mode and ACLs disabled.
const testInitSymlinksTemplate = `
version: v2
auth_server: example.com
outputs:
- type: identity
  destination:
    type: directory
    path: %s
    acls: off
    symlinks: %s
`

// TestInitSymlink tests tbot init with a symlink in the path.
func TestInitSymlink(t *testing.T) {
	secureWriteSupported, err := botfs.HasSecureWriteSupport()
	require.NoError(t, err)
	if !secureWriteSupported {
		t.Skip("Secure write not supported on this system.")
	}

	dir := t.TempDir()

	realPath := filepath.Join(dir, "data")
	dataDir := filepath.Join(dir, "data-symlink")
	require.NoError(t, os.Symlink(realPath, dataDir))

	// Should fail due to symlink in path.
	cfg := testConfigFromString(t, fmt.Sprintf(testInitSymlinksTemplate, dataDir, botfs.SymlinksSecure))
	require.Error(t, onInit(cfg, &config.CLIConf{}))

	// Should succeed when writing to the dir directly.
	cfg = testConfigFromString(t, fmt.Sprintf(testInitSymlinksTemplate, realPath, botfs.SymlinksSecure))
	require.NoError(t, onInit(cfg, &config.CLIConf{}))

	// Make sure everything was created.
	_ = validateFileDestination(t, cfg.Outputs[0])
}

// TestInitSymlinksInsecure should work on all platforms.
func TestInitSymlinkInsecure(t *testing.T) {
	dir := t.TempDir()

	realPath := filepath.Join(dir, "data")
	dataDir := filepath.Join(dir, "data-symlink")
	require.NoError(t, os.Symlink(realPath, dataDir))

	// Should succeed due to SymlinksInsecure
	cfg := testConfigFromString(t, fmt.Sprintf(testInitSymlinksTemplate, dataDir, botfs.SymlinksInsecure))
	require.Error(t, onInit(cfg, &config.CLIConf{}))
}
