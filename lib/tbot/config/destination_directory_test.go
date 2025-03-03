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

package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/utils"
)

func TestDestinationDirectory_Lock(t *testing.T) {
	dd := &DestinationDirectory{
		Path:     t.TempDir(),
		Symlinks: botfs.SymlinksInsecure,
		ACLs:     botfs.ACLOff,
	}

	// Successful lock
	unlock, err := dd.TryLock()
	require.NoError(t, err)

	// Another lock should fail
	_, err = dd.TryLock()
	require.ErrorIs(t, err, utils.ErrUnsuccessfulLockTry)

	// Release the lock
	require.NoError(t, unlock())

	// Trying to lock again should succeed
	unlock, err = dd.TryLock()
	require.NoError(t, err)

	// Release the lock
	require.NoError(t, unlock())
}

func TestDestinationDirectory_YAML(t *testing.T) {
	tests := []testYAMLCase[DestinationDirectory]{
		{
			name: "full",
			in: DestinationDirectory{
				Path:     "/my/path",
				ACLs:     botfs.ACLRequired,
				Symlinks: botfs.SymlinksSecure,
			},
		},
		{
			name: "minimal",
			in: DestinationDirectory{
				Path: "/my/path",
			},
		},
		{
			name: "acl readers",
			in: DestinationDirectory{
				Path: "/my/path",
				ACLs: botfs.ACLRequired,
				Readers: []*botfs.ACLSelector{
					{
						User: "foo",
					},
					{
						Group: "123",
					},
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestDestinationDirectory_ACLs(t *testing.T) {
	if !botfs.HasACLSupport() {
		t.Skipf("ACLs not supported on this host")
	}

	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "test")

	dd := DestinationDirectory{
		Path: path,
		ACLs: botfs.ACLRequired,
		Readers: []*botfs.ACLSelector{
			{
				// We explicitly want to support nonexistent UIDs, so we'll pick
				// an arbitrary UID we don't expect to exist. This particular
				// value is outside the usual reserved ranges documented by
				// Debian:
				// https://www.debian.org/doc/debian-policy/ch-opersys.html#uid-and-gid-classes
				User: "59123",
			},
		},
	}
	require.NoError(t, dd.CheckAndSetDefaults())

	err := dd.Init(ctx, []string{"foo"})
	if trace.IsNotImplemented(err) {
		t.Skipf("ACLs were unexpectedly not supported: %+v", err)
	}

	require.NoError(t, err)

	// An ACL should be configured for the root directory
	issues, err := botfs.VerifyACL(path, dd.Readers)
	require.NoError(t, err)
	require.Empty(t, issues)

	// Create an empty file to simulate an artifact. We explicitly want to
	// observe ACL problems from a file created out-of-band, so that Write() can
	// attempt to fix them.
	artifactPath := filepath.Join(path, "foo", "bar")
	f, err := os.Create(artifactPath)
	require.NoError(t, err)
	f.Close()

	issues, err = botfs.VerifyACL(artifactPath, dd.Readers)
	require.NoError(t, err)

	// note, lib/tbot/botfs's TestCompareACL tests the specifics of an empty ACL
	require.NotEmpty(t, issues)

	err = dd.Write(ctx, "foo/bar", []byte("hello world"))
	require.NoError(t, err)

	// The previous issues should now be resolved.
	issues, err = botfs.VerifyACL(artifactPath, dd.Readers)
	require.NoError(t, err)
	require.Empty(t, issues)

	// Finally, try writing to a completely separate path to ensure Write()
	// initializes new files with a sane ACL.
	require.NoError(t, dd.Write(ctx, "baz", []byte("example")))
	issues, err = botfs.VerifyACL(filepath.Join(path, "baz"), dd.Readers)
	require.NoError(t, err)
	require.Empty(t, issues)
}
