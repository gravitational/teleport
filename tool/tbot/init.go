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
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/cli"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

// RootUID is the UID of the root user
const RootUID = "0"

// aclTestFailedMessage is printed when an ACL test fails.
const aclTestFailedMessage = "ACLs are not usable for destination %s; " +
	"Change the destination's ACL mode to `off` to silence this warning."

// getInitArtifacts returns a map of all desired artifacts for the destination
func getInitArtifacts(target config.Initable) map[string]bool {
	// true = directory, false = regular file
	toCreate := map[string]bool{}

	// Collect all base artifacts and filter for the destination.
	for _, artifact := range identity.GetArtifacts() {
		if artifact.Matches(identity.DestinationKinds()...) {
			toCreate[artifact.Key] = false
		}
	}

	// Collect all config template artifacts.
	for _, fd := range target.Describe() {
		toCreate[fd.Name] = fd.IsDir
	}

	return toCreate
}

// getExistingArtifacts fetches all entries in a destination directory
func getExistingArtifacts(dir string) (map[string]bool, error) {
	existing := map[string]bool{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, entry := range entries {
		existing[entry.Name()] = entry.IsDir()
	}

	return existing, nil
}

// diffArtifacts computes the difference of two artifact sets
func diffArtifacts(a, b map[string]bool) map[string]bool {
	diff := map[string]bool{}

	for k, v := range a {
		if _, ok := b[k]; !ok {
			diff[k] = v
		}
	}

	return diff
}

// testACL creates a temporary file and attempts to apply an ACL to it. If the
// ACL is successfully applied, returns nil; otherwise, returns the error.
func testACL(directory string, ownerUser *user.User, opts *botfs.ACLOptions) error {
	// Note: we need to create the test file in the dest dir to ensure we
	// actually test the target filesystem.
	id, err := uuid.NewRandom()
	if err != nil {
		return trace.Wrap(err)
	}

	testFile := filepath.Join(directory, id.String())
	if err := botfs.Create(testFile, false, botfs.SymlinksInsecure); err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		err := os.Remove(testFile)
		if err != nil {
			log.DebugContext(
				context.TODO(),
				"Failed to delete ACL test file",
				"path", testFile,
			)
		}
	}()

	//nolint:staticcheck // staticcheck doesn't like nop implementations in fs_other.go
	if err := botfs.ConfigureLegacyACL(testFile, ownerUser, opts); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ensurePermissionsParams sets permissions options
type ensurePermissionsParams struct {
	// dirPath is the directory containing the file
	dirPath string

	// symlinksMode configures symlink attack mitigation behavior
	symlinksMode botfs.SymlinksMode

	// ownerUser is the user that should own the file
	ownerUser *user.User

	// ownerGroup is the group that should own the file
	ownerGroup *user.Group

	// aclOptions contains configuration for ACLs, if they are in use. nil if
	// ACLs are disabled
	aclOptions *botfs.ACLOptions

	// toCreate is a set of files that will be newly created, used to reduce
	// log spam when configuring new files
	toCreate map[string]bool
}

// ensurePermissions verifies permissions on the given path and, when
// possible, attempts to fix permissions / ACLs on any misconfigured paths.
func ensurePermissions(
	ctx context.Context,
	params *ensurePermissionsParams,
	key string,
	isDir bool,
) error {
	path := filepath.Join(params.dirPath, key)

	//nolint:staticcheck // this entirely innocuous line generates "related
	// information" lints for a false positive staticcheck lint relating to
	// nop function implementations in fs_other.go.
	stat, err := os.Stat(path)
	if err != nil {
		return trace.Wrap(err)
	}

	cleanPath := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return trace.Wrap(err)
	}

	// Precomputed flag to determine if certain log messages should be hidden.
	// We expect ownership, ACLs, etc to be wrong on initial create so warnings
	// are not desirable.
	_, newlyCreated := params.toCreate[key]
	verboseLogging := key != "" && !newlyCreated

	// This is unlikely as CreateSecure() refuses to follow symlinks, but users
	// could move things around after the fact.
	if cleanPath != resolved {
		switch params.symlinksMode {
		case botfs.SymlinksSecure:
			return trace.BadParameter("Path %q contains symlinks which is not "+
				"allowed for security reasons.", path)
		case botfs.SymlinksInsecure:
			// do nothing
		default:
			log.WarnContext(
				ctx,
				"Path contains symlinks and may be subject to symlink attacks. If this is intentional, consider setting `symlinks: insecure` in destination config",
				"path", path,
			)
		}
	}

	// A few conditions we won't try to handle...
	if isDir && !stat.IsDir() {
		return trace.BadParameter("File %s is expected to be a directory but is a file", path)
	} else if !isDir && stat.IsDir() {
		return trace.BadParameter("File %s is expected to be a file but is a directory", path)
	}

	currentUser, err := user.Current()
	if err != nil {
		return trace.Wrap(err)
	}

	uid, err := strconv.Atoi(params.ownerUser.Uid)
	if err != nil {
		return trace.Wrap(err)
	}

	gid, err := strconv.Atoi(params.ownerGroup.Gid)
	if err != nil {
		return trace.Wrap(err)
	}

	// Correct ownership.
	ownedByDesiredOwner, err := botfs.IsOwnedBy(stat, uid)
	if err != nil {
		log.DebugContext(ctx, "Could not determine file ownership", "path", path, "error", err)

		// Can't read file ownership on this platform (e.g. Windows), so always
		// attempt to chown (which does work on Windows)
		ownedByDesiredOwner = false
	}

	if !ownedByDesiredOwner {
		// If we're not running as root, this will probably fail.
		if currentUser.Uid != RootUID && runtime.GOOS != constants.WindowsOS {
			log.WarnContext(ctx, "Not running as root, ownership change is likely to fail")
		}

		if verboseLogging {
			log.WarnContext(
				ctx,
				"Ownership of file is incorrect and will be corrected",
				"path", path,
				"username", params.ownerUser.Username,
			)
		}

		err = os.Chown(path, uid, gid)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if params.aclOptions != nil {
		// We can verify ACLs as any user with read, but can only correct ACLs
		// as root or the owner.
		// Note that we rely on VerifyACL to return some error if permissions
		// are incorrect.

		//nolint:staticcheck // staticcheck doesn't like nop implementations in fs_other.go
		err = botfs.VerifyLegacyACL(path, params.aclOptions)
		//nolint:staticcheck // staticcheck doesn't like nop implementations in fs_other.go
		if err != nil && (currentUser.Uid == RootUID || currentUser.Uid == params.ownerUser.Uid) {
			if verboseLogging {
				log.WarnContext(
					ctx,
					"ACL for file is not correct and will be corrected",
					"path", path,
					"error", err,
				)
			}

			return trace.Wrap(botfs.ConfigureLegacyACL(path, params.ownerUser, params.aclOptions))
		} else if err != nil {
			log.ErrorContext(
				ctx,
				"ACL for file is incorrect but `tbot init` must be run as root or the owner to correct it",
				"path", path,
				"username", params.ownerUser.Username,
				"error", err,
			)
			return trace.AccessDenied("Elevated permissions required")
		}

		// ACL is valid.
		return nil
	}

	desiredMode := botfs.DefaultMode
	if stat.IsDir() {
		desiredMode = botfs.DefaultDirMode
	}

	// Using regular permissions, so attempt to correct the file mode.
	if stat.Mode().Perm() != desiredMode {
		if err := os.Chmod(path, desiredMode); err != nil {
			return trace.Wrap(err, "Could not fix permissions on file %q, expected %#o", path, desiredMode)
		}

		log.InfoContext(
			ctx,
			"Corrected permissions for file",
			"path", path,
			"from", stat.Mode().Perm(),
			"to", botfs.DefaultMode,
		)
	}

	return nil
}

// parseOwnerString parses and looks up an user and group from a user:group
// owner string.
func parseOwnerString(owner string) (*user.User, *user.Group, error) {
	ownerParts := strings.Split(owner, ":")
	if len(ownerParts) != 2 {
		return nil, nil, trace.BadParameter("invalid owner string: %q", owner)
	}

	ownerUser, err := user.Lookup(ownerParts[0])
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	ownerGroup, err := user.LookupGroup(ownerParts[1])
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return ownerUser, ownerGroup, nil
}

// getOwner determines the user/group owner given a CLI parameter (--owner)
// and desired default value. An empty defaultOwner defaults to the current
// user and their primary group.
func getOwner(cliOwner, defaultOwner string) (*user.User, *user.Group, error) {
	if cliOwner != "" {
		// If --owner is set, always use it.
		log.DebugContext(
			context.TODO(),
			"Attempting to use explicitly requested owner",
			"requested", cliOwner,
		)
		return parseOwnerString(cliOwner)
	}

	if defaultOwner != "" {
		log.DebugContext(
			context.TODO(),
			"Attempting to use default owner",
			"default", defaultOwner,
		)
		// If a default owner is specified, try it instead.
		return parseOwnerString(defaultOwner)
	}

	log.DebugContext(context.TODO(), "Will use current user as owner")
	// Otherwise, return the current user and group
	currentUser, err := user.Current()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	currentGroup, err := user.LookupGroupId(currentUser.Gid)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return currentUser, currentGroup, nil
}

// getAndTestACLOptions gets options needed to configure an ACL from CLI
// options and attempts to configure a test ACL to validate them. Ownership is
// not validated here.
func getAndTestACLOptions(initCmd *cli.InitCommand, destDir string) (*user.User, *user.Group, *botfs.ACLOptions, error) {
	if initCmd.BotUser == "" {
		return nil, nil, nil, trace.BadParameter("--bot-user must be set")
	}

	if initCmd.ReaderUser == "" {
		return nil, nil, nil, trace.BadParameter("--reader-user must be set")
	}

	botUser, err := user.Lookup(initCmd.BotUser)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	botGroup, err := user.LookupGroupId(botUser.Gid)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	readerUser, err := user.Lookup(initCmd.ReaderUser)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	opts := botfs.ACLOptions{
		BotUser:    botUser,
		ReaderUser: readerUser,
	}

	// Default to letting the bot own the destination, since by this point we
	// know the bot user definitely exists and is a reasonable owner choice.
	defaultOwner := fmt.Sprintf("%s:%s", botUser.Username, botGroup.Name)

	ownerUser, ownerGroup, err := getOwner(initCmd.Owner, defaultOwner)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	err = testACL(destDir, ownerUser, &opts)
	if err != nil && trace.IsAccessDenied(err) {
		return nil, nil, nil, trace.Wrap(err, "The destination %q does not appear to be writable", destDir)
	} else if err != nil {
		return nil, nil, nil, trace.Wrap(err, "ACL support may need to be enabled for the filesystem.")
	}

	return ownerUser, ownerGroup, &opts, nil
}

func onInit(globals *cli.GlobalArgs, init *cli.InitCommand) error {
	botConfig, err := cli.LoadConfigWithMutators(globals, init)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	initables := botConfig.GetInitables()
	var target config.Initable
	// First, resolve the correct output/service. If using a config file with
	// only 1 destination we can assume we want to init that one; otherwise,
	// --init-dir is required.
	if init.InitDir == "" {
		if len(initables) == 1 {
			target = initables[0]
		} else {
			return trace.BadParameter("An output or service to initialize must be specified with --init-dir")
		}
	} else {
		for _, v := range initables {
			d := v.GetDestination()
			dirDest, ok := d.(*config.DestinationDirectory)
			if ok && dirDest.Path == init.InitDir {
				target = v
				break
			}
		}
		if target == nil {
			return trace.NotFound("Initial directory %q must match a destination directory from the configuration file or --destination-dir parameter", init.InitDir)
		}
	}

	destDir, ok := target.GetDestination().(*config.DestinationDirectory)
	if !ok {
		return trace.BadParameter("`tbot init` only supports directory destinations")
	}

	log.InfoContext(ctx, "Initializing destination", "destination", destDir)

	// Create the directory if needed. We haven't checked directory ownership,
	// but it will fail when the ACLs are created if anything is misconfigured.
	if err := target.Init(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Next, test if we want + have ACL support, and set aclOpts if we do.
	// Desired owner is derived from the ACL mode.
	var aclOpts *botfs.ACLOptions
	var ownerUser *user.User
	var ownerGroup *user.Group

	switch destDir.ACLs {
	case botfs.ACLRequired, botfs.ACLTry:
		log.DebugContext(ctx, "Testing for ACL support")

		// Awkward control flow here, but we want these to fail together.
		ownerUser, ownerGroup, aclOpts, err = getAndTestACLOptions(init, destDir.Path)
		if err != nil {
			if destDir.ACLs == botfs.ACLRequired {
				// ACLs were specifically requested (vs "try" mode), so fail.
				return trace.Wrap(err, aclTestFailedMessage, destDir)
			}

			// Otherwise, fall back to no ACL with a warning.
			log.WarnContext(ctx, aclTestFailedMessage, "destination", destDir, "error", err)
			aclOpts = nil

			// We'll also need to re-fetch the owner as the defaults are
			// different in the fallback case.
			ownerUser, ownerGroup, err = getOwner(init.Owner, "")
			if err != nil {
				return trace.Wrap(err)
			}
		} else if aclOpts.ReaderUser.Uid == ownerUser.Uid {
			log.WarnContext(
				ctx,
				"The destination owner and reader are the same. This will break OpenSSH",
				"reader", aclOpts.ReaderUser.Username,
				"owner", ownerUser.Username,
			)
		}
	default:
		log.InfoContext(ctx, "ACLs disabled for this destination")
		ownerUser, ownerGroup, err = getOwner(init.Owner, "")
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Next, resolve what we want and what we already have.
	desired := getInitArtifacts(target)
	existing, err := getExistingArtifacts(destDir.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	toCreate := diffArtifacts(desired, existing)
	toRemove := diffArtifacts(existing, desired)

	// Based on this, create any new files.
	if len(toCreate) > 0 {
		log.InfoContext(ctx, "Attempting to create", "path", toCreate)

		for key, isDir := range toCreate {
			path := filepath.Join(destDir.Path, key)
			if err := botfs.Create(path, isDir, destDir.Symlinks); err != nil {
				return trace.Wrap(err)
			}

			log.InfoContext(ctx, "Created", "path", path)
		}
	} else {
		log.InfoContext(ctx, "Nothing to create.")
	}

	// ... and warn about / remove any unneeded files.
	if len(toRemove) > 0 && init.Clean {
		log.InfoContext(ctx, "Attempting to remove", "path", toRemove)

		var errors []error

		for key := range toRemove {
			path := filepath.Join(destDir.Path, key)

			if err := os.RemoveAll(path); err != nil {
				errors = append(errors, err)
			} else {
				log.InfoContext(ctx, "Removed", "path", path)
			}
		}

		if err := trace.NewAggregate(errors...); err != nil {
			return trace.Wrap(err)
		}
	} else if len(toRemove) > 0 {
		log.WarnContext(
			ctx,
			"Unexpected files found in destination directory, consider removing it manually or rerunning `tbot init` with the `--clean` flag",
		)
	} else {
		log.InfoContext(ctx, "Nothing to remove")
	}

	params := ensurePermissionsParams{
		dirPath:      destDir.Path,
		aclOptions:   aclOpts,
		ownerUser:    ownerUser,
		ownerGroup:   ownerGroup,
		symlinksMode: destDir.Symlinks,
		toCreate:     toCreate,
	}

	// Check and set permissions on the directory itself.
	if err := ensurePermissions(ctx, &params, "", true); err != nil {
		return trace.Wrap(err)
	}

	// Lastly, set and check permissions on all the desired files.
	for key, isDir := range desired {
		if err := ensurePermissions(ctx, &params, key, isDir); err != nil {
			return trace.Wrap(err)
		}
	}

	log.InfoContext(
		ctx,
		"Destination has been initialized. Note that these files will be empty and invalid until the bot issues certificates",
		"destination", destDir,
	)

	return nil
}
