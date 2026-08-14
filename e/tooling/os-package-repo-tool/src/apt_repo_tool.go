package main

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
)

type AptRepoTool struct {
	config       *AptConfig
	aptly        *Aptly
	gpg          *GPG
	s3Manager    *S3manager
	supportedOSs map[string][]string
	serializer   Serializer
	lockName     string
}

// Instantiates a new apt repo tool instance and performs any required setup/config.
func NewAptRepoTool(config *AptConfig, supportedOSs map[string][]string, serializer Serializer) (*AptRepoTool, error) {
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

	lockName, err := config.GetLockName("apt")
	if err != nil {
		return nil, trace.Wrap(err, "failed to get lock name")
	}

	return &AptRepoTool{
		aptly:        aptly,
		config:       config,
		gpg:          gpg,
		s3Manager:    s3Manager,
		supportedOSs: supportedOSs,
		serializer:   serializer,
		lockName:     lockName,
	}, nil
}

// Runs the tool, creating and updating APT repos based upon the current configuration.
func (art *AptRepoTool) Run() error {
	start := time.Now()
	slog.InfoContext(context.Background(), "Starting APT repo build process")
	slog.DebugContext(context.Background(), "Using providing configuration", "config", art.config)

	lockTimeoutCtx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
	defer cancel()
	releaseLock, err := art.serializer.TakeSerializationLock(lockTimeoutCtx, art.lockName)
	if err != nil {
		return trace.Wrap(err, "failed to take serialization lock")
	}
	// Release the lock and wait for the release to complete before we can return.
	defer func() { <-releaseLock() }()

	isFirstRun, err := art.aptly.IsFirstRun()
	if err != nil {
		return trace.Wrap(err, "failed to check if Aptly needs (re)built")
	}

	if isFirstRun {
		slog.WarnContext(context.Background(), "First run or disaster recovery detected, attempting to rebuild existing repos from APT repository")

		err = art.s3Manager.DownloadExistingRepo()
		if err != nil {
			return trace.Wrap(err, "failed to sync existing repo from S3 bucket")
		}

		_, err = art.recreateExistingRepos(art.config.localBucketPath)
		if err != nil {
			return trace.Wrap(err, "failed to recreate existing repos")
		}
	} else {
		slog.DebugContext(context.Background(), "Not first run of tool, skipping Aptly repository rebuild process")
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

	slog.InfoContext(context.Background(), "APT repo build process completed", "build_duration", time.Since(start).Round(time.Millisecond))
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
	slog.DebugContext(context.Background(), "Categorizing repos according to OS info", "repos", RepoNames(repos))
	categorizedRepos := make(map[string][]*Repo)
	for _, r := range repos {
		if osRepos, ok := categorizedRepos[r.OSInfo()]; ok {
			categorizedRepos[r.OSInfo()] = append(osRepos, r)
		} else {
			categorizedRepos[r.OSInfo()] = []*Repo{r}
		}
	}
	slog.DebugContext(context.Background(), "Categorized repos", "repos", categorizedRepos)

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
	slog.InfoContext(context.Background(), "Recreating previously published repos")
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

	slog.InfoContext(context.Background(), "Recreated and imported pre-existing artifacts for repos", "repo_count", len(createdRepos))
	return createdRepos, nil
}

func (art *AptRepoTool) getArtifactRepos() ([]*Repo, error) {
	slog.InfoContext(context.Background(), "Creating or getting Aptly repos for artifact requirements")

	artifactRepos, err := art.aptly.CreateReposFromArtifactRequirements(art.supportedOSs,
		art.config.releaseChannel, art.config.versionChannel)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create or get repos from artifact requirements")
	}

	slog.InfoContext(context.Background(), "Created or got artifact Aptly repos", "artifact_count", len(artifactRepos))
	return artifactRepos, nil
}

func (art *AptRepoTool) importNewDebs(repos []*Repo) error {
	slog.DebugContext(context.Background(), "Importing new debs into repos", "repo_count", len(repos), "repos", RepoNames(repos))
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
