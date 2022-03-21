package main

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type AptRepoTool struct {
	config       *Config
	aptly        *Aptly
	s3Manager    *s3manager
	supportedOSs map[string][]string
}

// Instantiates a new apt repo tool instance and performs any required setup/config.
func NewAptRepoTool(config *Config, supportedOSs map[string][]string) (*AptRepoTool, error) {
	art := &AptRepoTool{
		config:       config,
		s3Manager:    NewS3Manager(config.bucketName),
		supportedOSs: supportedOSs,
	}

	aptly, err := NewAptly()
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a new aptly instance")
	}
	art.aptly = aptly

	return art, nil
}

// Runs the tool, creating and updating APT repos based upon the current configuration.
func (art *AptRepoTool) Run() error {
	start := time.Now()
	logrus.Infoln("Starting APT repo build process...")

	err := art.s3Manager.DownloadExistingRepo(art.config.localBucketPath)
	if err != nil {
		return trace.Wrap(err, "failed to sync existing repo from S3 bucket")
	}

	repos, err := art.createRepos()
	if err != nil {
		return trace.Wrap(err, "failed to create repos")
	}

	err = art.importNewDebs(repos)
	if err != nil {
		return trace.Wrap(err, "failed to import new debs")
	}

	err = art.publishRepos(repos)
	if err != nil {
		return trace.Wrap(err, "failed to publish repos")
	}

	aptlyRootDir, err := art.aptly.GetRootDir()
	if err != nil {
		return trace.Wrap(err, "failed to get Aptly root directory")
	}

	err = art.s3Manager.UploadBuiltRepo(filepath.Join(aptlyRootDir, "public"))
	if err != nil {
		return trace.Wrap(err, "failed to sync changes to S3 bucket")
	}

	logrus.Infof("APT repo build process completed in %s\n", time.Since(start).Round(time.Millisecond))
	return nil
}

func (art *AptRepoTool) publishRepos(repos []*Repo) error {
	// Build a map keyed by os info with value of all repos that support the os in the key
	// This will be used to structure the publish command
	categorizedRepos := make(map[string][]*Repo)
	for _, r := range repos {
		if osRepos, ok := categorizedRepos[r.OSInfo()]; ok {
			categorizedRepos[r.OSInfo()] = append(osRepos, r)
		} else {
			categorizedRepos[r.OSInfo()] = []*Repo{r}
		}
	}

	for osInfo, osRepoList := range categorizedRepos {
		if len(osRepoList) < 1 {
			continue
		}

		err := art.aptly.PublishRepos(osRepoList, osRepoList[0].os)
		if err != nil {
			return trace.Wrap(err, "failed to publish for os %q", osInfo)
		}
	}

	return nil
}

func (art *AptRepoTool) recreateExistingRepos() ([]*Repo, error) {
	logrus.Infoln("Recreating previously published repos...")
	createdRepos, err := art.aptly.CreateReposFromPublishedPath(art.config.localBucketPath)
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

func (art *AptRepoTool) createRepos() ([]*Repo, error) {
	logrus.Infoln("Creating Aptly repos...")
	createdExistingRepos, err := art.recreateExistingRepos()
	if err != nil {
		return nil, trace.Wrap(err, "failed to recreate existing repos")
	}

	createdNewRepos, err := art.aptly.CreateReposFromArtifactRequirements(art.supportedOSs, art.config.releaseChannel, art.config.majorVersion)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create new repos from artifact requirements")
	}

	repos := append(createdExistingRepos, createdNewRepos...)
	logrus.Infof("Created %d Aptly repos\n", len(repos))
	return repos, nil
}

func (art *AptRepoTool) importNewDebs(repos []*Repo) error {
	logrus.Debugf("Importing new debs into %d repos: %q\n", len(repos), strings.Join(RepoNames(repos), "\", \""))
	err := filepath.WalkDir(art.config.artifactPath,
		func(debPath string, d fs.DirEntry, err error) error {
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
				if repo.majorVersion != art.config.majorVersion || repo.releaseChannel != art.config.releaseChannel {
					continue
				}

				err = art.aptly.ImportDeb(repo.Name(), debPath)
				if err != nil {
					return trace.Wrap(err, "failed to import deb from %s", debPath)
				}
			}

			return nil
		},
	)
	if err != nil {
		return trace.Wrap(err, "failed to find and import debs")
	}

	return nil
}
