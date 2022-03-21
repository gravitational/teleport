package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
)

type Repo struct {
	os                  string
	osVersion           string
	releaseChannel      string
	majorVersion        string
	publishedSourcePath string
}

// Returns a unique name for the repo.
func (r *Repo) Name() string {
	return fmt.Sprintf("%s-%s-%s-%s", r.os, r.osVersion, r.releaseChannel, r.majorVersion)
}

// Returns the APT component to be associated with all debs in the Aptly repo.
func (r *Repo) Component() string {
	return fmt.Sprintf("%s/%s", r.releaseChannel, r.majorVersion)
}

// Returns a string that identifies the specific OS and OS version the Aptly repo targets.
func (r *Repo) OSInfo() string {
	return fmt.Sprintf("%s %s", r.os, r.osVersion)
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
	return filepath.Join(r.publishedSourcePath, r.os, "dists", r.osVersion, r.releaseChannel, r.majorVersion), nil
}

// Helper function that calls `Name()` on all repos in the provided list.
func RepoNames(repos []*Repo) []string {
	repoNames := make([]string, len(repos))
	for i, repo := range repos {
		repoNames[i] = repo.Name()
	}

	return repoNames
}
