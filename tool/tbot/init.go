/*
Copyright2022 Gravitational, Inc.

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
	"path/filepath"

	"github.com/gravitational/teleport/tool/tbot/botfs"
	"github.com/gravitational/teleport/tool/tbot/config"
	"github.com/gravitational/teleport/tool/tbot/identity"
	"github.com/gravitational/trace"
)

func getInitArtifacts(destination *config.DestinationConfig) (map[string]bool, error) {
	//true = directory, false = regular file
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

// diffArtifacts computes the difference of two sets
func diffArtifacts(a, b map[string]bool) map[string]bool {
	diff := map[string]bool{}

	for k, v := range a {
		if _, ok := b[k]; !ok {
			diff[k] = v
		}
	}

	return diff
}

func ensurePermissions(path string, isDir bool, botUser string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return trace.Wrap(err)
	}

	lstat, err := os.Lstat(path)
	if err != nil {
		return trace.Wrap(err)
	}

	// This is unlikely as CreateSecure() refuses to follow symlinks, but users
	// could move things around after the fact.
	if !os.SameFile(stat, lstat) {
		return trace.BadParameter("File at %q is (or is under) a symlink which is not allowed for security reasons.")
	}

	if isDir && !stat.IsDir() {
		return trace.BadParameter("File %s is expected to be a directory but is a file")
	} else if !isDir && stat.IsDir() {
		return trace.BadParameter("File %s is expected to be a file but is a directory")
	}

	if err := botfs.VerifyACL(path, botUser); err != nil {
		return trace.Wrap(err)
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
			return trace.NotFound("Cannot initialize destination %q because it has not been configured.")
		}
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
		log.Warnf("Unexpected files found in destination directory, consider removing it manually or rerunning `tbot init` with the `--clean` flag.")
	} else {
		log.Info("Nothing to remove.")
	}

	// Lastly, set and check permissions on all the desired files.
	for key, isDir := range desired {
		path := filepath.Join(destDir.Path, key)
		if err := ensurePermissions(path, isDir, cf.BotUser); err != nil {
			return trace.Wrap(err)
		}
	}

	log.Infof("Destination %s has been initialized. Note that these files will be empty and invalid until the bot issues certificates.", destImpl)

	return nil
}
