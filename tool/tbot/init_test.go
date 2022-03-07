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
	"os/user"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/tool/tbot/botfs"
	"github.com/gravitational/teleport/tool/tbot/config"
	"github.com/gravitational/teleport/tool/tbot/identity"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// usernamesToTry contains a list of usernames we can use as ACL targets in
// testing.
var usernamesToTry = []string{"nobody", "ci", "root"}

func contains(entries []string, entry string) bool {
	for _, e := range entries {
		if e == entry {
			return true
		}
	}

	return false
}

// filterUsers returns the input list of usernames except for those in the
// exclude list.
func filterUsers(usernames, exclude []string) []string {
	ret := []string{}

	for _, username := range usernames {
		if !contains(exclude, username) {
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

// initACLTest performs checks to ensure we can test ACLs on this system. If
// the system doesn't support ACLs, the test is skipped, but if the ACL test
// succeeds, returns a usable ACLOptions. Other errors may still fail the test.
func initACLTest(t *testing.T) (*botfs.ACLOptions, error) {
	hasACLSupport, err := botfs.HasACLSupport()
	require.NoError(t, err)

	if !hasACLSupport {
		return nil, trace.NotImplemented("Platform has no ACL support")
	}

	opts, err := getACLOptions()
	if trace.IsNotImplemented(err) {
		t.Skipf("%+v", err)
	} else if trace.IsNotFound(err) {
		t.Skipf("%+v", err)
	}
	require.NoError(t, err)

	err = testACL(t.TempDir(), opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return opts, nil
}

// testConfig creates a BotConfig from the given CLI config.
func testConfig(t *testing.T, cf *config.CLIConf) *config.BotConfig {
	cfg, err := config.FromCLIConf(cf)
	require.NoError(t, err)

	return cfg
}

// validateFileDestinations ensures all files in a destination exist on disk as
// expected, and returns the destination.
func validateFileDestination(t *testing.T, dest *config.DestinationConfig) *config.DestinationDirectory {
	destImpl, err := dest.GetDestination()
	require.NoError(t, err)

	destDir, ok := destImpl.(*config.DestinationDirectory)
	require.True(t, ok)

	for _, art := range identity.GetArtifacts() {
		if !art.Matches(dest.Kinds...) {
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
	cfg := testConfig(t, cf)

	// Run init.
	require.NoError(t, onInit(cfg, cf))

	// Make sure everything was created.
	_ = validateFileDestination(t, cfg.Destinations[0])
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

	// Determine if we expect init to use ACLs.
	expectACLs := false
	if hasACLSupport {
		if err := testACL(t.TempDir(), opts); err == nil {
			expectACLs = true
		}
	}

	currentUser, err := user.Current()
	require.NoError(t, err)

	currentGroup, err := user.LookupGroupId(currentUser.Gid)
	require.NoError(t, err)

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
		Owner: fmt.Sprintf("%s:%s", currentUser.Name, currentGroup.Name),
	}
	cfg := testConfig(t, cf)

	// Run init.
	require.NoError(t, onInit(cfg, cf))

	// Make sure everything was created.
	destDir := validateFileDestination(t, cfg.Destinations[0])

	// If we expected ACLs, verify them:
	if expectACLs {
		require.NoError(t, destDir.Verify(identity.ListKeys(cfg.Destinations[0].Kinds...)))
	} else {
		t.Logf("Skipping ACL check on %q as they should not be supported.", dir)
	}
}
