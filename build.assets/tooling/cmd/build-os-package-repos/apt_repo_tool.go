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
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type AptRepoTool struct {
	config       *AptConfig
	aptly        *Aptly
	gpg          *GPG
	s3Manager    *S3manager
	supportedOSs map[string][]string
}

// Instantiates a new apt repo tool instance and performs any required setup/config.
func NewAptRepoTool(config *AptConfig, supportedOSs map[string][]string) (*AptRepoTool, error) {
	aptly, err := NewAptly(config.aptlyPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a new aptly instance")
	}

	gpg, err := NewGPG()
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a new GPG instance")
	}

	s3Manager, err := NewS3Manager(config.S3Config)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a new s3manager instance")
	}

	return &AptRepoTool{
		aptly:        aptly,
		config:       config,
		gpg:          gpg,
		s3Manager:    s3Manager,
		supportedOSs: supportedOSs,
	}, nil
}

// Runs the tool, creating and updating APT repos based upon the current configuration.
func (art *AptRepoTool) Run() error {
	start := time.Now()
	logrus.Infoln("Starting APT repo build process...")
	logrus.Debugf("Using config: %+v", spew.Sdump(art.config))

	isFirstRun, err := art.aptly.IsFirstRun()
	if err != nil {
		return trace.Wrap(err, "failed to check if Aptly needs (re)built")
	}

	if isFirstRun {
		logrus.Warningln("First run or disaster recovery detected, attempting to rebuild existing repos from APT repository...")

		err = art.s3Manager.DownloadExistingRepo()
		if err != nil {
			return trace.Wrap(err, "failed to sync existing repo from S3 bucket")
		}

		_, err = art.recreateExistingRepos(art.config.localBucketPath)
		if err != nil {
			return trace.Wrap(err, "failed to recreate existing repos")
		}
	} else {
		logrus.Debugf("Not first run of tool, skipping Aptly repository rebuild process")
	}

	// Note: this logic will only push the artifact into the `art.supportedOSs` repos.
	// This behavior is intended to allow deprecating old OS versions in the future
	// without removing the associated repos entirely.
	artifactRepos, err := art.getArtifactRepos()
	if err != nil {
		return trace.Wrap(err, "failed to create repos")
	}

	err = art.importNewDebs(artifactRepos)
	if err != nil {
		return trace.Wrap(err, "failed to import new debs")
	}

	err = art.publishRepos()
	if err != nil {
		return trace.Wrap(err, "failed to publish repos")
	}

	// Both Hashicorp and Docker publish their key to this path
	err = art.gpg.WritePublicKeyToFile(filepath.Join(art.aptly.rootDir, "public", "gpg"))
	if err != nil {
		return trace.Wrap(err, "failed to write GPG public key")
	}

	art.s3Manager.ChangeLocalBucketPath(filepath.Join(art.aptly.rootDir, "public"))
	err = art.s3Manager.UploadBuiltRepo()
	if err != nil {
		return trace.Wrap(err, "failed to sync changes to S3 bucket")
	}

	// Future work: add literals to config?
	err = art.s3Manager.UploadRedirectURL("index.html", "https://goteleport.com/docs/installation/#linux")
	if err != nil {
		return trace.Wrap(err, "failed to redirect index page to Teleport docs")
	}

	logrus.Infof("APT repo build process completed in %s", time.Since(start).Round(time.Millisecond))
	return nil
}

func (art *AptRepoTool) publishRepos() error {
	// Pull in all Aptly repos, not just the latest ones to ensure they all get built into APT repos correctly
	repos, err := art.aptly.GetAllRepos()
	if err != nil {
		return trace.Wrap(err, "failed to get all Aptly repos")
	}

	// Build a map keyed by os info with value of all repos that support the os in the key
	// This will be used to structure the publish command
	logrus.Debugf("Categorizing repos according to OS info: %v", RepoNames(repos))
	categorizedRepos := make(map[string][]*Repo)
	for _, r := range repos {
		if osRepos, ok := categorizedRepos[r.OSInfo()]; ok {
			categorizedRepos[r.OSInfo()] = append(osRepos, r)
		} else {
			categorizedRepos[r.OSInfo()] = []*Repo{r}
		}
	}
	logrus.Debugf("Categorized repos: %v", categorizedRepos)

	for osInfo, osRepoList := range categorizedRepos {
		if len(osRepoList) < 1 {
			continue
		}

		err := art.aptly.PublishRepos(osRepoList, osRepoList[0].os, osRepoList[0].osVersion)
		if err != nil {
			return trace.Wrap(err, "failed to publish for os %q", osInfo)
		}
	}

	return nil
}

func (art *AptRepoTool) recreateExistingRepos(localPublishedPath string) ([]*Repo, error) {
	logrus.Infoln("Recreating previously published repos...")
	createdRepos, err := art.aptly.CreateReposFromPublishedPath(localPublishedPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to recreate existing repos")
	}

	for _, repo := range createdRepos {
		err := art.aptly.ImportDebsFromExistingRepo(repo)
		if err != nil {
			return nil, trace.Wrap(err, "failed to import debs from existing repo %q", repo.Name())
		}
	}

	logrus.Infof("Recreated and imported pre-existing artifacts for %d repos", len(createdRepos))
	return createdRepos, nil
}

func (art *AptRepoTool) getArtifactRepos() ([]*Repo, error) {
	logrus.Infoln("Creating or getting Aptly repos for artifact requirements...")

	artifactRepos, err := art.aptly.CreateReposFromArtifactRequirements(art.supportedOSs,
		art.config.releaseChannel, art.config.versionChannel)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create or get repos from artifact requirements")
	}

	logrus.Infof("Created or got %d artifact Aptly repos", len(artifactRepos))
	return artifactRepos, nil
}

func (art *AptRepoTool) importNewDebs(repos []*Repo) error {
	logrus.Debugf("Importing new debs into %d repos: %q", len(repos), strings.Join(RepoNames(repos), "\", \""))
	err := filepath.WalkDir(art.config.artifactPath,
		func(debPath string, d fs.DirEntry, err error) error {
			return art.importNewDebsWalker(debPath, d, err, repos)
		},
	)
	if err != nil {
		return trace.Wrap(err, "failed to find and import debs")
	}

	return nil
}

// This should not be used outside of importNewDebs
func (art *AptRepoTool) importNewDebsWalker(debPath string, d fs.DirEntry, err error, repos []*Repo) error {
	if err != nil {
		return trace.Wrap(err, "failure while searching %s for debs", debPath)
	}

	if d.IsDir() {
		return nil
	}

	fileName := d.Name()
	if filepath.Ext(fileName) != ".deb" {
		return nil
	}

	// Import new artifacts into all repos that match the artifact's requirements
	for _, repo := range repos {
		// Other checks could be added here to ensure that a given deb gets added to the correct repo
		// such as name or parent directory, facilitating os-specific artifacts
		if repo.versionChannel != art.config.versionChannel || repo.releaseChannel != art.config.releaseChannel {
			continue
		}

		err = art.aptly.ImportDeb(repo.Name(), debPath)
		if err != nil {
			return trace.Wrap(err, "failed to import deb from %s", debPath)
		}
	}

	return nil
}
