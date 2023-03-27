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
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

type Repo struct {
	os                  string
	osVersion           string
	releaseChannel      string
	versionChannel      string
	publishedSourcePath string
}

// Returns a unique name for the repo.
func (r *Repo) Name() string {
	return fmt.Sprintf("%s-%s-%s-%s", r.os, r.osVersion, r.releaseChannel, r.versionChannel)
}

func NewRepoFromName(name string) (*Repo, error) {
	splitName := strings.Split(name, "-")
	if len(splitName) != 4 {
		return nil, trace.Errorf("the provided repo name %q is not a valid repo name", name)
	}

	return &Repo{
		os:             splitName[0],
		osVersion:      splitName[1],
		releaseChannel: splitName[2],
		versionChannel: splitName[3],
	}, nil
}

// Returns the APT component to be associated with all debs in the Aptly repo.
func (r *Repo) Component() string {
	return fmt.Sprintf("%s/%s", r.releaseChannel, r.versionChannel)
}

// Returns a string that identifies the specific OS and OS version the Aptly repo targets.
func (r *Repo) OSInfo() string {
	return fmt.Sprintf("%s/%s", r.os, r.osVersion)
}

// Returns true if the repo is a recreation of a published repo, false otherewise.
// If true, publishedSourcePath is a valid existing path on the filesystem.
func (r *Repo) WasCreatedFromPublishedSource() (bool, error) {
	if r.publishedSourcePath == "" {
		return false, nil
	}

	_, err := os.Stat(r.publishedSourcePath)
	if os.IsNotExist(err) {
		return false, trace.Errorf("the published source path of repo %q was not empty (%q), but does not exist on disk", r.Name(), r.publishedSourcePath)
	}

	return true, nil
}

// Returns the absolute path to the published repo on disk that this repo was created from.
func (r *Repo) PublishedRepoAbsolutePath() (string, error) {
	wasCreatedFromPublishedSource, err := r.WasCreatedFromPublishedSource()
	if err != nil {
		return "", trace.Wrap(err, "failed to verify if the repo %q is a recreation of an existing published repo", r.Name())
	}

	if !wasCreatedFromPublishedSource {
		return "", trace.Errorf("repo %q was not created from a publish source and therefor has no published source path", r.Name())
	}

	// `/<publishedSourcePath>/<os>/dists/<os version>/<release channel>/<major version>/`
	return filepath.Join(r.publishedSourcePath, r.os, "dists", r.osVersion, r.releaseChannel, r.versionChannel), nil
}

// Helper function that calls `Name()` on all repos in the provided list.
func RepoNames(repos []*Repo) []string {
	repoNames := make([]string, len(repos))
	for i, repo := range repos {
		repoNames[i] = repo.Name()
	}

	return repoNames
}
