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
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

// This provides wrapper functions for the Aptly command. Aptly is written in Go but it doesn't appear
// to have a good binary API to use, only a CLI tool and REST API.

type Aptly struct {
	rootDir string
}

// Instantiates Aptly, performing any system configuration needed.
func NewAptly(rootDir string) (*Aptly, error) {
	a := &Aptly{
		rootDir: rootDir,
	}

	err := a.ensureDefaultConfigExists()
	if err != nil {
		return nil, trace.Wrap(err, "failed to ensure Aptly default config exists")
	}

	err = a.updateConfiguration()
	if err != nil {
		return nil, trace.Wrap(err, "failed to load Aptly configuration")
	}

	return a, nil
}

func (*Aptly) ensureDefaultConfigExists() error {
	// If the default config doesn't exist then it will be created the first time an Aptly command is
	// ran, which messes up the output.
	// Note: it is important to not use any repo-related commands here as they have a side effect of
	// also creating the Aptly rootDir structure which is usually undesirable here
	_, err := BuildAndRunCommand("aptly", "config", "show")
	if err != nil {
		return trace.Wrap(err, "failed to create default Aptly config")
	}

	return nil
}

func (a *Aptly) updateConfiguration() error {
	aptlyConfigMap, err := loadAptlyConfigMap()
	if err != nil {
		return trace.Wrap(err, "failed to load Aptly config map")
	}

	// Additional config can be handled here if needed in the future
	aptlyConfigMap["rootDir"] = a.rootDir

	logrus.Debugf("Built Aptly config: %v", aptlyConfigMap)
	saveAptlyConfigMap(aptlyConfigMap)

	configOutput, err := BuildAndRunCommand("aptly", "config", "show")
	if err != nil {
		return trace.Wrap(err, "failed to check Aptly config")
	}
	logrus.Debugln("Aptly config on disk:")
	logrus.Debugf("%v", configOutput)

	return nil
}

func saveAptlyConfigMap(aptlyConfigMap map[string]interface{}) error {
	aptlyConfigData, err := json.MarshalIndent(aptlyConfigMap, "", "  ")
	if err != nil {
		return trace.Wrap(err, "failed to marshal Aptly config with data %v", aptlyConfigMap)
	}

	aptlyConfigPath, err := getAptlyConfigPath()
	if err != nil {
		return trace.Wrap(err, "failed to get Aptly config path")
	}

	err = os.WriteFile(aptlyConfigPath, aptlyConfigData, 0644)
	if err != nil {
		return trace.Wrap(err, "failed to write Aptly config to %q with data %q", aptlyConfigPath, string(aptlyConfigData))
	}

	return nil
}

func loadAptlyConfigMap() (map[string]interface{}, error) {
	aptlyConfigPath, err := getAptlyConfigPath()
	if err != nil {
		return nil, trace.Wrap(err, "failed to get Aptly config path")
	}

	aptlyConfigData, err := os.ReadFile(aptlyConfigPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read Aptly config file at %q", aptlyConfigPath)
	}

	var aptlyConfigMap map[string]interface{}
	if err := json.Unmarshal(aptlyConfigData, &aptlyConfigMap); err != nil {
		return nil, trace.Wrap(err, "failed to unmarshal Aptly config from file %q with data %q", aptlyConfigPath, string(aptlyConfigData))
	}

	return aptlyConfigMap, nil
}

// This follows the config logic specified here: https://www.aptly.info/doc/configuration/
func getAptlyConfigPath() (string, error) {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", trace.Wrap(err, "failed to get user home directory path")
	}

	userAptlyConfigPath := path.Join(userHomeDir, ".aptly.conf")
	_, err = os.Stat(userAptlyConfigPath)
	if err == nil {
		return userAptlyConfigPath, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", trace.Wrap(err, "failed to check if %q exists", userAptlyConfigPath)
	}

	systemAptlyConfigPath := "/etc/aptly.conf"
	_, err = os.Stat(systemAptlyConfigPath)
	if err == nil {
		return systemAptlyConfigPath, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", trace.Wrap(err, "failed to check if %q exists", systemAptlyConfigPath)
	}

	return "", trace.Errorf("Aptly config not found at %q or %q", userAptlyConfigPath, systemAptlyConfigPath)
}

func (a *Aptly) IsFirstRun() (bool, error) {
	_, err := os.Stat(a.rootDir)
	if err == nil {
		return false, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}

	return false, trace.Wrap(err, "failed to check if %q exists", a.rootDir)
}

// Creates the provided repo `r` via Aptly. Returns true if the repo was created, false otherewise.
func (a *Aptly) CreateRepoIfNotExists(r *Repo) (bool, error) {
	logrus.Debugf("Creating repo %q if it doesn't already exist..", r.Name())
	doesRepoExist, err := a.DoesRepoExist(r)
	if err != nil {
		return false, trace.Wrap(err, "failed to check whether or not the repo %q already exists", r.Name())
	}

	if doesRepoExist {
		logrus.Debugf("Repo %q already exists, skipping creation", r.Name())
		return false, nil
	}

	distributionArg := fmt.Sprintf("-distribution=%s", r.osVersion)
	componentArg := fmt.Sprintf("-component=%s/%s", r.releaseChannel, r.versionChannel)
	_, err = BuildAndRunCommand("aptly", "repo", "create", distributionArg, componentArg, r.Name())
	if err != nil {
		return false, trace.Wrap(err, "failed to create repo %q", r.Name())
	}

	logrus.Debugf("Created repo %q", r.Name())
	return true, nil
}

// Checks to see if the Aptly described by repo `r` exists. Returns true if it exists, false otherwise.
func (a *Aptly) DoesRepoExist(r *Repo) (bool, error) {
	logrus.Debugf("Checking if repo %q exists...", r.Name())

	existingRepoNames, err := a.GetExistingRepoNames()
	if err != nil {
		return false, trace.Wrap(err, "failed to get existing repo names")
	}

	return slices.Contains(existingRepoNames, r.Name()), nil
}

// Gets a list of the name of Aptly repos that already exists.
func (a *Aptly) GetExistingRepoNames() ([]string, error) {
	logrus.Debugln("Getting a list of pre-existing repos...")
	// The output of the command will be simiar to:
	// ```
	// <repo name 1>
	// ...
	// <repo name N>
	// ```
	output, err := BuildAndRunCommand("aptly", "repo", "list", "-raw")
	if err != nil {
		return nil, trace.Wrap(err, "failed to get a list of existing repos")
	}

	// Split the command output by new line
	parsedRepoNames := strings.Split(output, "\n")

	// The names may have whitespace and the command may print an extra blank line, so we remove those here
	var validRepoNames []string
	for _, parsedRepoName := range parsedRepoNames {
		if trimmedRepoName := strings.TrimSpace(parsedRepoName); trimmedRepoName != "" {
			validRepoNames = append(validRepoNames, trimmedRepoName)
		}
	}

	logrus.Debugf("Found %d repos: %q", len(validRepoNames), strings.Join(validRepoNames, "\", \""))
	return validRepoNames, nil
}

// Imports a deb at `debPath` into the Aptly repo of name `repoName`.
// If `debPath` is a folder, the folder will be searched recursively for *.deb files
// which are then imported into the repo.
func (a *Aptly) ImportDeb(repoName string, debPath string) error {
	logrus.Infof("Importing deb(s) from %q into repo %q...", debPath, repoName)

	_, err := BuildAndRunCommand("aptly", "repo", "add", repoName, debPath)
	if err != nil {
		return trace.Wrap(err, "failed to add %q to repo %q", debPath, repoName)
	}

	return nil
}

// This function imports deb files from a preexisting published repo, typically created from a previous run of this tool.
func (a *Aptly) ImportDebsFromExistingRepo(repo *Repo) error {
	logrus.Infof("Importing pre-existing debs from repo %q...", repo.Name())
	publishedRepoAbsolutePath, err := repo.PublishedRepoAbsolutePath()
	if err != nil {
		return trace.Wrap(err, "failed to get the absolute path of the published repo %q", repo.Name())
	}

	logrus.Debugf("Looking in %q for Packages files...", publishedRepoAbsolutePath)
	err = filepath.WalkDir(publishedRepoAbsolutePath,
		func(packagesPath string, d fs.DirEntry, err error) error {
			if err != nil {
				return trace.Wrap(err, "failure while searching %s for Packages files", packagesPath)
			}

			if d.IsDir() {
				return nil
			}

			if d.Name() != "Packages" {
				return nil
			}

			logrus.Debugf("Matched %q as a Packages file, attempting to import listed debs into %q...", packagesPath, repo.Name())
			err = a.importDebsFromPackagesFile(repo, packagesPath)
			if err != nil {
				return trace.Wrap(err, "failed to import debs into repo %q from packages file %q", repo.Name(), packagesPath)
			}

			return nil
		},
	)

	if err != nil {
		return trace.Wrap(err, "failed to find and import debs from existing repo %q at published path %q", repo.Name(), publishedRepoAbsolutePath)
	}

	return nil
}

func (a *Aptly) importDebsFromPackagesFile(repo *Repo, packagesPath string) error {
	logrus.Debugf("Importing debs from %q into %q", packagesPath, repo.Name())
	debRelativeFilePaths, err := parsePackagesFile(packagesPath)
	if err != nil {
		return trace.Wrap(err, "failed to parse packages file %q for deb file paths", packagesPath)
	}

	logrus.Debugf("Found %d debs listed in %q: %q", len(debRelativeFilePaths), packagesPath, strings.Join(debRelativeFilePaths, "\", \""))
	for _, debRelativeFilePath := range debRelativeFilePaths {
		debPath := path.Join(repo.publishedSourcePath, repo.os, debRelativeFilePath)
		logrus.Debugf("Constructed deb absolute path %q", debPath)
		err = a.ImportDeb(repo.Name(), debPath)
		if err != nil {
			return trace.Wrap(err, "failed to import deb into repo %q from %q", repo.Name(), debPath)
		}
	}

	return nil
}

func parsePackagesFile(packagesPath string) ([]string, error) {
	logrus.Debugf("Parsing packages file %q", packagesPath)
	file, err := os.Open(packagesPath)
	if err != nil {
		logrus.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	debRelativeFilePaths := make([]string, 0, 1) // If the package file exists then it should contain at least one deb file path
	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines
		if line == "" {
			continue
		}

		key, value, err := parsePackagesFileLine(line)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse line %q in packages file %q", line, packagesPath)
		}

		if key != "Filename" {
			continue
		}

		logrus.Debugf("Found deb file listed at relative path %q", value)
		debRelativeFilePaths = append(debRelativeFilePaths, value)
	}

	if err := scanner.Err(); err != nil {
		return nil, trace.Wrap(err, "received error while reading %q", packagesPath)
	}

	return debRelativeFilePaths, nil
}

func parsePackagesFileLine(line string) (string, string, error) {
	splitLine := strings.SplitN(line, ": ", 2)
	if len(splitLine) != 2 {
		return "", "", trace.Errorf("packages file line %q is malformed", line)
	}

	key := splitLine[0]
	value := splitLine[1]

	if key == "" {
		return "", "", trace.Errorf("packages file line %q contains an empty key", line)
	}

	if value == "" {
		return "", "", trace.Errorf("packages file line %q contains an empty value", line)
	}

	return key, value, nil
}

// Publishes the Aptly repos defined in the `repos` slice to the `repoOS` subpath.
func (a *Aptly) PublishRepos(repos []*Repo, repoOS string, repoOSVersion string) error {
	repoNames := RepoNames(repos)
	logrus.Infof("Publishing repos for OS %q: %q...", repoOS, strings.Join(repoNames, "\", \""))

	// Trying to publish to an already published OS/OS version will fail, and dropping a published
	// OS/OS version when no new components (release channel/major version) are added is
	// computationally expensive
	areSomeUnpublished, areSomePublished, err := a.getRepoSlicePublishedState(repos)
	if err != nil {
		return trace.Wrap(err, "failed to determine if repos for  have been published or not")
	}

	logrus.Debugln("Repo OS/OS version combo publish state:")
	logrus.Debugf("Are some unpublished: %v", areSomeUnpublished)
	logrus.Debugf("Are some published: %v", areSomePublished)
	logrus.Debugf("Repos: %v", RepoNames(repos))

	// If all repos have been published
	if areSomePublished && !areSomeUnpublished {
		// Update rather than republish
		_, err := BuildAndRunCommand("aptly", "publish", "update", repoOSVersion, repoOS)
		if err != nil {
			return trace.Wrap(err, "failed to update publish repos with OS %q and OS version %q", repoOS, repoOSVersion)
		}

		return nil
	}

	// If some have been published and some have not
	// This will occur if there is a new major release, a OS version is supported, or a new release channel is added
	if areSomePublished && areSomeUnpublished {
		// Drop the currently published APT repo so that it can be rebuilt from scratch
		_, err := BuildAndRunCommand("aptly", "publish", "drop", repoOSVersion, repoOS)
		if err != nil {
			return trace.Wrap(err, "failed to update publish repos with OS %q and OS version %q", repoOS, repoOSVersion)
		}
	}

	// If all repos have not been published (or were just unpublished/dropped)
	// Build the command args and publish all the repos
	args := []string{"publish", "repo"}
	if len(repos) > 1 {
		componentsArgument := fmt.Sprintf("-component=%s", strings.Repeat(",", len(repos)-1))
		args = append(args, componentsArgument)
	}
	args = append(args, repoNames...)
	args = append(args, repoOS)

	// Full command is `aptly publish repo -component=<, repeating len(repos) - 1 times> <repo names> <repo OS>`
	_, err = BuildAndRunCommand("aptly", args...)
	if err != nil {
		return trace.Wrap(err, "failed to publish repos")
	}

	return nil
}

// This function determines if `repos` contains repos that have an OS/OS version combo
// that have not yet been published, and if it contains repos that have an OS/OS version
// combo that have been published.
//
// Returns:
//
// 1. true if `repos` contains at least one repo who's OS/OS version combo has not been
// published yet
//
// 2. true if `repos` contains at least one repo who's OS/OS version combo has been
// published
//
// Example:
//
// `getRepoSlicePublishedState([<repo for debian-trixie-stable-v6>,
// <repo for debian-buster-stable-v7>])` will return
//
// 1. `true, false, nil` if a repo for debian/trixie, debian/buster has not been published yet
//
// 2. `false, true, nil` if a repo for debian/trixie, debian/buster has been published
//
// 3. `true, true, nil` if a repo for debian/trixie has not been published yet but debian/buster
// has, or vice versa
func (a *Aptly) getRepoSlicePublishedState(repos []*Repo) (bool, bool, error) {
	publishedRepoNames, err := a.GetPublishedRepoNames()
	if err != nil {
		return false, false, trace.Wrap(err, "failed to get a list of published repos' names")
	}

	containsUnpublishedRepo := false
	containsPublishedRepo := false
	for _, repo := range repos {
		repoName := repo.Name()
		hasRepoBeenPublished := slices.Contains(publishedRepoNames, repoName)
		logrus.Debugf("Repo %q has been published: %v", repoName, hasRepoBeenPublished)
		containsUnpublishedRepo = containsUnpublishedRepo || !hasRepoBeenPublished
		containsPublishedRepo = containsPublishedRepo || hasRepoBeenPublished

		// No need to keep checking if they're both already true
		if containsUnpublishedRepo && containsPublishedRepo {
			break
		}
	}

	// failsafe in case of some underlying logic change
	if !containsUnpublishedRepo && !containsPublishedRepo {
		return false, false, trace.Errorf("something went very wrong here")
	}

	return containsUnpublishedRepo, containsPublishedRepo, nil
}

func (a *Aptly) GetPublishedRepoNames() ([]string, error) {
	logrus.Debugln("Getting a list of published repos...")
	// The output of the command will be simiar to:
	// ```
	// Published repositories:
	//   * <os 1>/<osVersion 1> [<APT repo supported arch 1>, ..., <APT repo supported arch M>] publishes {<APT repo component 1>: [<Aptly repo name>]}, ..., {<APT repo component N>: [<Aptly repo name>]}
	// ...
	//   * <os 1>/<osVersion J> [<APT repo supported arch 1>, ..., <APT repo supported arch M>] publishes {<APT repo component 1>: [<Aptly repo name>]}, ..., {<APT repo component N>: [<Aptly repo name>]}
	//   * <os 2>/<osVersion 1> [<APT repo supported arch 1>, ..., <APT repo supported arch M>] publishes {<APT repo component 1>: [<Aptly repo name>]}, ..., {<APT repo component N>: [<Aptly repo name>]}
	// ...
	//   * <os I>/<osVersion J> [<APT repo supported arch 1>, ..., <APT repo supported arch M>] publishes {<APT repo component 1>: [<Aptly repo name>]}, ..., {<APT repo component N>: [<Aptly repo name>]}
	// ```
	//
	// If no repos have been published then the output will be similar to:
	// ```
	// No snapshots/local repos have been published. Publish a snapshot by running `aptly publish snapshot ...`.
	// ```
	// Note that the `-raw` argument is not used here as it does not provide sufficient information
	output, err := BuildAndRunCommand("aptly", "publish", "list")
	if err != nil {
		return nil, trace.Wrap(err, "failed to get a list of published repos")
	}

	// Split the command output by new line
	publishedRepoLines := strings.Split(output, "\n")
	// In all cases the first line should exist and not be parsed
	publishedRepoLines = publishedRepoLines[1:]

	repoNameRegexStr := ": \\[(.+?)\\]"
	repoNameRegex, err := regexp.Compile(repoNameRegexStr)
	if err != nil {
		return nil, trace.Wrap(err, "failed to compile repo name regex %q", repoNameRegexStr)
	}

	var publishedRepoNames []string
	for _, publishedRepoLine := range publishedRepoLines {
		// The names may have whitespace and the command may print an extra blank line, so we remove those here
		if trimmedRepoLine := strings.TrimSpace(publishedRepoLine); trimmedRepoLine != "" {
			// Additional parsing should go here if needed in the future
			repoNameMatches := repoNameRegex.FindAllStringSubmatch(publishedRepoLine, -1)
			if repoNameMatches == nil {
				return nil, trace.Errorf("failed to match repo names in line %q with regex %q", publishedRepoLine, repoNameRegexStr)
			}

			for _, repoNameMatch := range repoNameMatches {
				// `repoNameRegexStr` is written such that there will be exactly one match and one group in repoNameMatch
				// for example repoNameMatch could be [": [debian-bookworm-stable-v6]", "debian-bookworm-stable-v6"]
				publishedRepoName := repoNameMatch[1]
				publishedRepoNames = append(publishedRepoNames, publishedRepoName)
			}
		}
	}

	logrus.Debugf("Found %d published repos: %q", len(publishedRepoNames), publishedRepoNames)
	return publishedRepoNames, nil
}

// Creates Aptly repos from a local path that has previously published Apt repos created by this tool.
// Returns a list of repo objects describing the created Aptly repos.
func (a *Aptly) CreateReposFromPublishedPath(localPublishedPath string) ([]*Repo, error) {
	// The file tree that we care about here will be of the following structure:
	// `/<bucketPath>/<os>/dists/<os version>/<release channel>/<major version>/...`
	logrus.Infof("Recreating previously published repos from %q...", localPublishedPath)

	createdRepos := []*Repo{}

	osSubdirectories, err := getSubdirectories(localPublishedPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get OS subdirectories in %s", localPublishedPath)
	}

	for _, os := range osSubdirectories {
		osVersionParentDirectory := filepath.Join(localPublishedPath, os, "dists")
		osVersionSubdirectories, err := getSubdirectories(osVersionParentDirectory)
		if err != nil {
			return nil, trace.Wrap(err, "failed to get OS version subdirectories in %s", localPublishedPath)
		}

		for _, osVersion := range osVersionSubdirectories {
			releaseChannelParentDirectory := filepath.Join(osVersionParentDirectory, osVersion)
			releaseChannelSubdirectories, err := getSubdirectories(releaseChannelParentDirectory)
			if err != nil {
				return nil, trace.Wrap(err, "failed to get release channel subdirectories in %s", localPublishedPath)
			}

			for _, releaseChannel := range releaseChannelSubdirectories {
				versionChannelParentDirectory := filepath.Join(releaseChannelParentDirectory, releaseChannel)
				versionChannelSubdirectories, err := getSubdirectories(versionChannelParentDirectory)
				if err != nil {
					return nil, trace.Wrap(err, "failed to get version channel subdirectories in %s", localPublishedPath)
				}

				for _, versionChannel := range versionChannelSubdirectories {
					r := &Repo{
						os:                  os,
						osVersion:           osVersion,
						releaseChannel:      releaseChannel,
						versionChannel:      versionChannel,
						publishedSourcePath: localPublishedPath,
					}

					wasRepoCreated, err := a.CreateRepoIfNotExists(r)
					if err != nil {
						return nil, trace.Wrap(err, "failed to create repo %q", r.Name())
					}

					if wasRepoCreated {
						createdRepos = append(createdRepos, r)
					}
				}
			}
		}
	}

	logrus.Infof("Recreated %d repos", len(createdRepos))
	return createdRepos, nil
}

// Creates or gets Aptly repos for all permutations of the provided requirements.
// Returns a list of repo objects describing the Aptly repos, regardless of if they
// already existed.
//
// supportedOSInfo should be a dictionary keyed by OS name, with values being a list of
// supported OS version codenames.
func (a *Aptly) CreateReposFromArtifactRequirements(supportedOSInfo map[string][]string,
	releaseChannel string, versionChannel string) ([]*Repo, error) {
	logrus.Infoln("Creating new repos from artifact requirements:")
	logrus.Infof("Supported OSs: %+v", supportedOSInfo)
	logrus.Infof("Release channel: %q", releaseChannel)
	logrus.Infof("Version channel: %q", versionChannel)

	artifactRequirementRepos := []*Repo{}
	for os, osVersions := range supportedOSInfo {
		for _, osVersion := range osVersions {
			r := &Repo{
				os:             os,
				osVersion:      osVersion,
				releaseChannel: releaseChannel,
				versionChannel: versionChannel,
			}

			_, err := a.CreateRepoIfNotExists(r)
			if err != nil {
				return nil, trace.Wrap(err, "failed to create repo %q", r.Name())
			}

			artifactRequirementRepos = append(artifactRequirementRepos, r)
		}
	}

	return artifactRequirementRepos, nil
}

// Returns a list of all Aptly reported repos
func (a *Aptly) GetAllRepos() ([]*Repo, error) {
	repoNames, err := a.GetExistingRepoNames()
	if err != nil {
		return nil, trace.Wrap(err, "failed to get existing repo names")
	}

	repos := make([]*Repo, len(repoNames))
	for i, repoName := range repoNames {
		repo, err := NewRepoFromName(repoName)
		if err != nil {
			return nil, trace.Wrap(err, "failed to build repo struct for repo name %q", repoName)
		}

		repos[i] = repo
	}

	return repos, nil
}

func getSubdirectories(basePath string) ([]string, error) {
	logrus.Debugf("Getting subdirectories of %q...", basePath)
	files, err := os.ReadDir(basePath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read directory %q", basePath)
	}

	subdirectories := []string{}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		subdirectory := file.Name()
		logrus.Debugf("Found subdirectory %q", subdirectory)
		subdirectories = append(subdirectories, subdirectory)
	}

	return subdirectories, nil
}
