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

package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/cli"
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

// testConfigFromString parses a YAML config file from a string.
func testConfigFromString(t *testing.T, yamlStr string) *config.BotConfig {
	// Load YAML to validate syntax.
	cfg, err := config.ReadConfig(strings.NewReader(yamlStr), false)
	require.NoError(t, err)
	require.NoError(t, cfg.CheckAndSetDefaults())

	// Reencode as a string
	out := &strings.Builder{}
	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	err = enc.Encode(cfg)
	require.NoError(t, err)

	// Load and return the static config
	globalArgs := cli.NewGlobalArgsWithStaticConfig(out.String())
	cfg, err = cli.LoadConfigWithMutators(globalArgs)
	require.NoError(t, err)

	return cfg
}

// validateFileDestinations ensures all files in a destination exist on disk as
// expected, and returns the destination.
func validateFileDestination(t *testing.T, svc config.Initable) *config.DestinationDirectory {
	destImpl := svc.GetDestination()

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
	cmd := &cli.InitCommand{
		LegacyDestinationDirArgs: &cli.LegacyDestinationDirArgs{
			DestinationDir: dir,
		},
		AuthProxyArgs: cli.NewStaticAuthServer("example.com"),
	}

	// Run init.
	require.NoError(t, onInit(&cli.GlobalArgs{}, cmd))

	cfg, err := cli.LoadConfigWithMutators(&cli.GlobalArgs{}, cmd)
	require.NoError(t, err)

	// Make sure everything was created.
	_ = validateFileDestination(t, cfg.GetInitables()[0])
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

	currentUser, err := user.Current()
	require.NoError(t, err)

	currentGroup, err := user.LookupGroupId(currentUser.Gid)
	require.NoError(t, err)

	// Determine if we expect init to use ACLs.
	expectACLs := false
	if botfs.HasACLSupport() {
		if err := testACL(t.TempDir(), currentUser, opts); err == nil {
			expectACLs = true
		}
	}

	// Note: we'll use the current user as owner as that's the only way to
	// guarantee ACL write access.
	dir := t.TempDir()
	cmd := &cli.InitCommand{
		LegacyDestinationDirArgs: &cli.LegacyDestinationDirArgs{
			DestinationDir: dir,
		},
		BotUser:    opts.BotUser.Username,
		ReaderUser: opts.ReaderUser.Username,

		// This isn't a default, but unfortunately we need to specify a
		// non-nobody owner for CI purposes.
		Owner: fmt.Sprintf("%s:%s", currentUser.Username, currentGroup.Name),

		AuthProxyArgs: cli.NewStaticAuthServer("example.com"),
	}

	cfg, err := cli.LoadConfigWithMutators(&cli.GlobalArgs{}, cmd)
	require.NoError(t, err)

	// Run init.
	require.NoError(t, onInit(&cli.GlobalArgs{}, cmd))

	// Make sure everything was created.
	destDir := validateFileDestination(t, cfg.GetInitables()[0])

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
	if !botfs.HasSecureWriteSupport() {
		t.Skip("Secure write not supported on this system.")
	}

	dir := t.TempDir()

	realPath := filepath.Join(dir, "data")
	dataDir := filepath.Join(dir, "data-symlink")
	require.NoError(t, os.Symlink(realPath, dataDir))

	// Should fail due to symlink in path.
	cfgStr := fmt.Sprintf(testInitSymlinksTemplate, dataDir, botfs.SymlinksSecure)
	globals := cli.NewGlobalArgsWithStaticConfig(cfgStr)
	require.Error(t, onInit(globals, &cli.InitCommand{}))

	// Should succeed when writing to the dir directly.
	cfgStr = fmt.Sprintf(testInitSymlinksTemplate, realPath, botfs.SymlinksSecure)
	globals = cli.NewGlobalArgsWithStaticConfig(cfgStr)
	require.NoError(t, onInit(globals, &cli.InitCommand{}))

	// Make sure everything was created. We'll have to rebuild the config from
	// scratch since we don't have a copy available.
	cfg := testConfigFromString(t, cfgStr)
	_ = validateFileDestination(t, cfg.GetInitables()[0])
}

// TestInitSymlinksInsecure should work on all platforms.
func TestInitSymlinkInsecure(t *testing.T) {
	dir := t.TempDir()

	realPath := filepath.Join(dir, "data")
	dataDir := filepath.Join(dir, "data-symlink")
	require.NoError(t, os.Symlink(realPath, dataDir))

	// Should succeed due to SymlinksInsecure

	globals := cli.NewGlobalArgsWithStaticConfig(fmt.Sprintf(testInitSymlinksTemplate, dataDir, botfs.SymlinksInsecure))
	require.Error(t, onInit(globals, &cli.InitCommand{}))
}
