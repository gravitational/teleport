package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// This provides wrapper functions for the Aptly command. Aptly is written in Go but it doesn't appear
// to have a good binary API to use, only a CLI tool and REST API.

type Aptly struct{}

// Instantiates Aptly, performing any system configuration needed.
func NewAptly() (*Aptly, error) {
	a := &Aptly{}
	err := a.ensureDefaultConfigExists()
	if err != nil {
		return nil, trace.Wrap(err, "failed to ensure Aptly default config exists")
	}
	// Additional config can be handled here if needed in the future
	return a, nil
}

func (*Aptly) ensureDefaultConfigExists() error {
	// If the default config doesn't exist then it will be created the first time an Aptly command is
	// ran, which messes up the output.
	_, err := buildAndRunCommand("aptly", "repo", "list")
	if err != nil {
		return trace.Wrap(err, "failed to create default Aptly config")
	}

	return nil
}

// Creates the provided repo `r` via Aptly. Returns true if the repo was created, false otherewise.
func (a *Aptly) CreateRepoIfNotExists(r *Repo) (bool, error) {
	logrus.Debugf("Creating repo %q if it doesn't already exist...\n", r.Name())
	doesRepoExist, err := a.DoesRepoExist(r)
	if err != nil {
		return false, trace.Wrap(err, "failed to check whether or not the repo %q already exists", r.Name())
	}

	if doesRepoExist {
		logrus.Debugf("Repo %q already exists, skipping creation\n", r.Name())
		return false, nil
	}

	distributionArg := fmt.Sprintf("-distribution=%s", r.osVersion)
	componentArg := fmt.Sprintf("-component=%s/%s", r.releaseChannel, r.majorVersion)
	_, err = buildAndRunCommand("aptly", "repo", "create", distributionArg, componentArg, r.Name())
	if err != nil {
		return false, trace.Wrap(err, "failed to create repo %q", r.Name())
	}

	logrus.Debugf("Created repo %q\n", r.Name())
	return true, nil
}

// Checks to see if the Aptly described by repo `r` exists. Returns true if it exists, false otherwise.
func (a *Aptly) DoesRepoExist(r *Repo) (bool, error) {
	repoName := r.Name()
	logrus.Debugf("Checking if repo %q exists...\n", repoName)

	existingRepoNames, err := a.GetExistingRepoNames()
	if err != nil {
		return false, trace.Wrap(err, "failed to get existing repo names")
	}

	for _, existingRepoName := range existingRepoNames {
		if repoName == existingRepoName {
			logrus.Debugf("Match found: %q matches %q\n", existingRepoName, repoName)
			return true, nil
		}
		logrus.Debugf("Did not match %q as %q\n", existingRepoName, repoName)
	}

	logrus.Debugf("Match not found for repo %q\n", repoName)
	return false, nil
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
	output, err := buildAndRunCommand("aptly", "repo", "list", "-raw")
	if err != nil {
		return nil, trace.Wrap(err, "failed to get a list of existing repos")
	}

	// Split the command output by new line
	parsedRepoNames := strings.Split(output, "\n")

	// The names may have whitespace and the command may print an extra blank line, so we remove those here
	var validRepoNames []string
	for _, parsedRepoName := range parsedRepoNames {
		if trimedRepoName := strings.TrimSpace(parsedRepoName); trimedRepoName != "" {
			validRepoNames = append(validRepoNames, trimedRepoName)
		}
	}

	logrus.Debugf("Found %d repos: %q\n", len(validRepoNames), strings.Join(validRepoNames, "\", \""))
	return validRepoNames, nil
}

// Imports a deb at `debPath` into the Aptly repo of name `repoName`.
// If `debPath` is a folder, the folder will be searched recursively for *.deb files
// which are then imported into the repo.
func (a *Aptly) ImportDeb(repoName string, debPath string) error {
	logrus.Infof("Importing deb(s) from %q into repo %q...\n", debPath, repoName)

	_, err := buildAndRunCommand("aptly", "repo", "add", repoName, debPath)
	if err != nil {
		return trace.Wrap(err, "failed to add %q to repo %q", debPath, repoName)
	}

	return nil
}

// This function imports deb files from a preexisting published repo, typically created from a previous run of this tool.
func (a *Aptly) ImportDebsFromExistingRepo(repo *Repo) error {
	logrus.Infof("Importing pre-existing debs from repo %q...\n", repo.Name())
	publishedRepoAbsolutePath, err := repo.PublishedRepoAbsolutePath()
	if err != nil {
		return trace.Wrap(err, "failed to get the absolute path of the published repo %q", repo.Name())
	}

	logrus.Debugf("Looking in %q for Packages files...\n", publishedRepoAbsolutePath)
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

			logrus.Debugf("Matched %q as a Packages file, attempting to import listed debs into %q\n...", packagesPath, repo.Name())
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
	logrus.Debugf("Importing debs from %q into %q\n", packagesPath, repo.Name())
	debRelativeFilePaths, err := parsePackagesFile(packagesPath)
	if err != nil {
		return trace.Wrap(err, "failed to parse packages file %q for deb file paths", packagesPath)
	}

	logrus.Debugf("Found %d debs listed in %q: %q\n", len(debRelativeFilePaths), packagesPath, strings.Join(debRelativeFilePaths, "\", \""))
	for _, debRelativeFilePath := range debRelativeFilePaths {
		debPath := path.Join(repo.publishedSourcePath, repo.os, debRelativeFilePath)
		logrus.Debugf("Constructed deb absolute path %q\n", debPath)
		err = a.ImportDeb(repo.Name(), debPath)
		if err != nil {
			return trace.Wrap(err, "failed to import deb into repo %q from %q", repo.Name(), debPath)
		}
	}

	return nil
}

func parsePackagesFile(packagesPath string) ([]string, error) {
	logrus.Debugf("Parsing packages file %q\n", packagesPath)
	file, err := os.Open(packagesPath)
	if err != nil {
		log.Fatal(err)
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

		logrus.Debugf("Found deb file listed at relative path %q\n", value)
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
func (a *Aptly) PublishRepos(repos []*Repo, repoOS string) error {
	repoNames := RepoNames(repos)
	logrus.Infof("Publishing repos for OS %q: %q...\n", repoOS, strings.Join(repoNames, "\", \""))

	// Build the args
	args := []string{"publish", "repo"}
	if len(repos) > 1 {
		componentsArgument := fmt.Sprintf("-component=%s", strings.Repeat(",", len(repos)-1))
		args = append(args, componentsArgument)
	}
	args = append(args, repoNames...)
	args = append(args, repoOS)

	// Full command is `aptly publish repo -component=<, repeating len(repos) - 1 times> <repo names> <repo OS>`
	_, err := buildAndRunCommand("aptly", args...)
	if err != nil {
		return trace.Wrap(err, "failed to publish repos")
	}

	return nil
}

// Returns the Aptly root dir. This is usually `~/.aptly`, but may vary based upon config.
func (a *Aptly) GetRootDir() (string, error) {
	logrus.Debugln("Checking Aptly config for the Aptly root directory...")
	output, err := buildAndRunCommand("aptly", "config", "show")

	if err != nil {
		return "", trace.Wrap(err, "failed retrieve Aptly config info")
	}

	var outputJson map[string]interface{}
	err = json.Unmarshal([]byte(output), &outputJson)
	if err != nil {
		return "", trace.Wrap(err, "failed to unmarshal `%s` output JSON into map", output)
	}

	if rootDirValue, ok := outputJson["rootDir"]; !ok {
		return "", trace.Errorf("Failed to find `rootDir` key in `%s` output JSON", output)
	} else {
		if rootDirString, ok := rootDirValue.(string); !ok {
			return "", trace.Errorf("The `rootDir` key in `%s` output JSON is not of type `string`", output)
		} else {
			logrus.Debugf("Found Aptly root directory at %q\n", rootDirString)
			return rootDirString, nil
		}
	}
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
				majorVersionParentDirectory := filepath.Join(releaseChannelParentDirectory, releaseChannel)
				majorVersionSubdirectories, err := getSubdirectories(majorVersionParentDirectory)
				if err != nil {
					return nil, trace.Wrap(err, "failed to get major version subdirectories in %s", localPublishedPath)
				}

				for _, majorVersion := range majorVersionSubdirectories {
					r := &Repo{
						os:                  os,
						osVersion:           osVersion,
						releaseChannel:      releaseChannel,
						majorVersion:        majorVersion,
						publishedSourcePath: localPublishedPath,
					}

					wasRepoCreated, err := a.CreateRepoIfNotExists(r)
					if err != nil {
						return nil, trace.Wrap(err, "failed to create repo %q", r.Name())
					}

					if wasRepoCreated {
						createdRepos = append(createdRepos, r)
						logrus.Infof("Created repo %q", r.Name())
					} else {
						logrus.Debugf("Repo %q already exists, skipping creation", r.Name())
					}
				}
			}
		}
	}

	logrus.Infof("Recreated %d repos", len(createdRepos))
	return createdRepos, nil
}

// Creates Aptly repos for all permutations of the provided requirements.
// Returns a list of repo objects describing the created Aptly repos.
// supportedOSInfo should be a dictionary keyed by OS name, with values being a list of
//   supported OS version codenames.
func (a *Aptly) CreateReposFromArtifactRequirements(supportedOSInfo map[string][]string,
	releaseChannel string, majorVersion string) ([]*Repo, error) {
	logrus.Infoln("Creating new repos from artifact requirements:")
	logrus.Infof("Supported OSs: %+v\n", supportedOSInfo)
	logrus.Infof("Release channel: %q\n", releaseChannel)
	logrus.Infof("Artifact major version: %q\n", majorVersion)

	createdRepos := []*Repo{}
	for os, osVersions := range supportedOSInfo {
		for _, osVersion := range osVersions {
			r := &Repo{
				os:             os,
				osVersion:      osVersion,
				releaseChannel: releaseChannel,
				majorVersion:   majorVersion,
			}

			wasRepoCreated, err := a.CreateRepoIfNotExists(r)
			if err != nil {
				return nil, trace.Wrap(err, "failed to create repo %q", r.Name())
			}

			if wasRepoCreated {
				createdRepos = append(createdRepos, r)
				logrus.Infof("Created repo %q", r.Name())
			} else {
				logrus.Debugf("Repo %q already exists, skipping creation", r.Name())
			}
		}
	}

	return createdRepos, nil
}

func getSubdirectories(basePath string) ([]string, error) {
	logrus.Debugf("Getting subdirectories of %q\n...", basePath)
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read directory %q", basePath)
	}

	subdirectories := []string{}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		subdirectory := file.Name()
		logrus.Debugf("Found subdirectory %q\n", subdirectory)
		subdirectories = append(subdirectories, subdirectory)
	}

	return subdirectories, nil
}

func buildAndRunCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	logrus.Debugf("Running command \"%s '%s'\"\n", command, strings.Join(args, "' '"))
	output, err := cmd.CombinedOutput()

	if output != nil {
		logrus.Debugf("Command output:\n%s\n", string(output))
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode := exitError.ExitCode()
			logrus.Debugf("Command exited with exit code %d\n", exitCode)
		} else {
			logrus.Debugln("Command failed without an exit code")
		}
		return "", trace.Wrap(err, "Command failed, see debug output for additional details")
	}

	logrus.Debugln("Command exited successfully")
	return string(output), nil
}
