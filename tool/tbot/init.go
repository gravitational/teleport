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
	"os"
	"os/user"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/tool/tbot/botfs"
	"github.com/gravitational/teleport/tool/tbot/config"
	"github.com/gravitational/teleport/tool/tbot/identity"
	"github.com/gravitational/trace"
)

const aclTestFailedMessage = "ACLs are not usable for destination %s; " +
	"consider enabling ACL support for the filesystem or changing the " +
	"destination's ACL mode to `off`"

// getInitArtifacts returns a map of all desired artifacts for the destination
func getInitArtifacts(destination *config.DestinationConfig) (map[string]bool, error) {
	// true = directory, false = regular file
	toCreate := map[string]bool{}

	// Collect all base artifacts and filter for the destination.
	for _, artifact := range identity.GetArtifacts() {
		if artifact.Matches(destination.Kinds...) {
			toCreate[artifact.Key] = false
		}
	}

	// Collect all config template artifacts.
	for _, templateConfig := range destination.Configs {
		template, err := templateConfig.GetConfigTemplate()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, file := range template.Describe() {
			toCreate[file.Name] = file.IsDir
		}
	}

	return toCreate, nil
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
func testACL(directory string, botUser *user.User) error {
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
			log.Debugf("Failed to delete ACL test file %q", testFile)
		}
	}()

	if err := botfs.ConfigureACL(testFile, botUser.Uid); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ensurePermissions verifies permissions on the given path and, when
// possible, attempts to fix permissions / ACLs on any misconfigured paths.
func ensurePermissions(
	dirPath string, key string, isDir bool, botUser *user.User,
	symlinksMode botfs.SymlinksMode, useACLs bool, toCreate map[string]bool,
) error {
	path := filepath.Join(dirPath, key)

	stat, err := os.Stat(path)
	if err != nil {
		return trace.Wrap(err)
	}

	cleanPath := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return trace.Wrap(err)
	}

	// This is unlikely as CreateSecure() refuses to follow symlinks, but users
	// could move things around after the fact.
	if cleanPath != resolved {
		switch symlinksMode {
		case botfs.SymlinksSecure:
			return trace.BadParameter("Path %q contains symlinks which is not "+
				"allowed for security reasons.", path)
		case botfs.SymlinksInsecure:
			// do nothing
		default:
			log.Warnf("Path %q contains symlinks and may be subject to symlink "+
				"attacks. If this is intentional, consider setting `symlinks: "+
				"insecure` in destination config.", path)
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

	// Note: it's unlikely we can chown(), so we'll just warn if ownership is
	// wrong.
	ownedByCurrentUser, err := botfs.IsOwnedBy(stat, currentUser)
	if err == nil && !ownedByCurrentUser {
		log.Warnf("File %q is not owned by the current user, permission operations are likely to fail.", path)
	}

	if useACLs && botUser.Uid != currentUser.Uid {
		// Only bother noting specific problems if the file already existed.
		_, newlyCreated := toCreate[key]
		if key != "" && !newlyCreated {
			if err := botfs.VerifyACL(path, botUser.Uid); err != nil {
				log.Warnf("ACL for %q is not correct and will be corrected: %v", path, err)
			}
		}

		// Regardless of whether there are problems with the ACL, will set it
		// anyway.
		return trace.Wrap(botfs.ConfigureACL(path, botUser.Uid))
	} else if useACLs && botUser.Uid == currentUser.Uid {
		// Admittedly ugly control flow, but ACLs are useless in this case, so just warn.
		log.Warn("Bot user is the current user so ACLs will not be set. " +
			"If this is intentional, be aware it will reduce security of " +
			"stored certificates. Set `acls: off` in the destination config " +
			"to disable this warning.")
	}

	// Attempt to correct the file mode.
	if stat.Mode().Perm() != botfs.DefaultMode {
		if err := os.Chmod(path, botfs.DefaultMode); err != nil {
			return trace.Wrap(err, "Could not fix permissions on file %q", path)
		}

		log.Infof("Corrected permissions on %q from %#o to %#o", path, stat.Mode().Perm(), botfs.DefaultMode)
	}

	return nil
}

func onInit(botConfig *config.BotConfig, cf *config.CLIConf) error {
	var destination *config.DestinationConfig
	var err error

	// First, resolve the correct destination. If using a config file with only
	// 1 destination we can assume we want to init that one; otherwise,
	// --init-dir is required.
	if cf.InitDir == "" {
		if len(botConfig.Destinations) == 1 {
			destination = botConfig.Destinations[0]
		} else {
			return trace.BadParameter("A destination to initialize must be specified with --init-dir")
		}
	} else {
		destination, err = botConfig.GetDestinationByPath(cf.InitDir)
		if err != nil {
			return trace.WrapWithMessage(err, "Could not find specified destination %q", cf.InitDir)
		}

		if destination == nil {
			// TODO: in the future if/when other backends are supported,
			// destination might be nil because the user tried to enter a non
			// filesystem path, so this error message could be misleading.
			return trace.NotFound("Cannot initialize destination %q because "+
				"it has not been configured.", cf.InitDir)
		}
	}

	botUser, err := user.Lookup(cf.BotUser)
	if err != nil {
		return trace.Wrap(err)
	}

	destImpl, err := destination.GetDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	destDir, ok := destImpl.(*config.DestinationDirectory)
	if !ok {
		return trace.BadParameter("`tbot init` only supports directory destinations")
	}

	log.Infof("Initializing destination: %s", destImpl)

	// Create the directory if needed.
	// TODO: verify ownership of this directory matches current user
	if err := destDir.Init(); err != nil {
		return trace.Wrap(err)
	}

	// Next, test if we have ACL support
	useACLs := false
	switch destDir.ACLs {
	case botfs.ACLOn, botfs.ACLTry:
		useACLs = true

		log.Debug("Testing for ACL support...")
		err := testACL(destDir.Path, botUser)
		if err != nil {
			if destDir.ACLs == botfs.ACLOn {
				return trace.Wrap(err, aclTestFailedMessage, destImpl)
			}

			log.WithError(err).Warnf(aclTestFailedMessage, destImpl)
			useACLs = false
		}
	default:
		log.Info("ACLs disabled for this destination.")
	}

	// Next, resolve what we want and what we already have.
	desired, err := getInitArtifacts(destination)
	if err != nil {
		return trace.Wrap(err)
	}

	existing, err := getExistingArtifacts(destDir.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	toCreate := diffArtifacts(desired, existing)
	toRemove := diffArtifacts(existing, desired)

	// Based on this, create any new files.
	if len(toCreate) > 0 {
		log.Infof("To create: %v", toCreate)

		for key, isDir := range toCreate {
			path := filepath.Join(destDir.Path, key)
			if err := botfs.Create(path, isDir, destDir.Symlinks); err != nil {
				return trace.Wrap(err)
			}

			log.Infof("Created: %s", path)
		}
	} else {
		log.Info("Nothing to create.")
	}

	// ... and warn about / remove any unneeded files.
	if len(toRemove) > 0 && cf.Clean {
		log.Infof("To remove: %v", toRemove)
		for key := range toRemove {
			path := filepath.Join(destDir.Path, key)

			if err := os.RemoveAll(path); err != nil {
				return trace.Wrap(err)
			}
			log.Infof("Removed: %s", path)
		}
	} else if len(toRemove) > 0 {
		log.Warnf("Unexpected files found in destination directory, consider " +
			"removing it manually or rerunning `tbot init` with the `--clean` " +
			"flag.")
	} else {
		log.Info("Nothing to remove.")
	}

	// Check and set permissions on the directory itself.
	if err := ensurePermissions(destDir.Path, "", true, botUser, destDir.Symlinks, useACLs, toCreate); err != nil {
		return trace.Wrap(err)
	}

	// Lastly, set and check permissions on all the desired files.
	for key, isDir := range desired {
		if err := ensurePermissions(destDir.Path, key, isDir, botUser, destDir.Symlinks, useACLs, toCreate); err != nil {
			return trace.Wrap(err)
		}
	}

	log.Infof("Destination %s has been initialized. Note that these files "+
		"will be empty and invalid until the bot issues certificates.",
		destImpl)

	return nil
}
