package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/gravitational/teleport/.cloudbuild/scripts/internal/github"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// Config represents a git repository that has been configured to use a
// deployment key, and acts as a handle to the resources so that we can
// clean them up when we're done.
type Config struct {
	identity   string
	knownHosts string
	repoDir    string
}

// Configure alters the configuration of the git repository in `repoDir`
// so that we can access it from build. If `deployKey` is non-nil the repo
// will be configured to use that. If no deploy key is supplied, the repository
// config is untouched.
func Configure(ctx context.Context, repoDir string, deployKey []byte) (cfg *Config, err error) {
	var identity string
	var hostsFile string

	// The deploy key we're using is too sensitive to just hope that every
	// exit path will clean it up on failure, so let's just register a
	// cleanup function now just in case.
	defer func() {
		if err != nil {
			cleanup(identity, hostsFile)
		}
	}()

	// If we have been supplied with deployment key, then we need to use that
	// to access the repository. This implies that need to use ssh to read from
	// the remote, so we will have to deal with known_hosts, etc.
	if deployKey != nil {
		githubHostKeys, err := getGithubHostKeys(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "failed configuring known hosts")
		}

		// configure SSH known_hosts for github.com
		hostsFile, err = configureKnownHosts("github.com", githubHostKeys)
		if err != nil {
			return nil, trace.Wrap(err, "failed configuring known hosts")
		}

		// GCB clones via https, and our deploy key won't work with that, so
		// force git to access the remote over ssh with a rewrite rule
		log.Info("Adding remote url rewrite rule")
		err = git(ctx, repoDir, "config", `url.git@github.com:.insteadOf`, "https://github.com/")
		if err != nil {
			return nil, trace.Wrap(err, "failed configuring url rewrite rule")
		}

		// set up the identity for the deploy key
		identity, err = writeKey(deployKey)
		if err != nil {
			return nil, trace.Wrap(err, "failed configuring SSH identity")
		}

		// finally, force git to
		//   a) use our custom SSH setup when accessing the remote, and
		//   b) fail if the github host tries to present a host key other than
		//      one we got from the metadata service
		log.Infof("Configuring git ssh command")
		err = git(ctx, repoDir, "config", "core.sshCommand",
			fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=yes -o UserKnownHostsFile=%s", identity, hostsFile))
		if err != nil {
			return nil, trace.Wrap(err, "failed configuring git to use deploy key")
		}
	}

	return &Config{identity: identity, repoDir: repoDir, knownHosts: hostsFile}, nil
}

// Do runs `git args...` in the configured repository
func (cfg *Config) Do(ctx context.Context, args ...string) error {
	return git(ctx, cfg.repoDir, args...)
}

// Close cleans up the repository, including deleting the deployment key (if any)
func (cfg *Config) Close() error {
	cleanup(cfg.identity, cfg.knownHosts)
	return nil
}

func run(ctx context.Context, workingDir string, env []string, cmd string, args ...string) error {
	p := exec.CommandContext(ctx, cmd, args...)
	if len(env) != 0 {
		p.Env = append(os.Environ(), env...)
	}
	p.Dir = workingDir

	cmdLogger := log.WithField("cmd", cmd)
	p.Stdout = cmdLogger.WriterLevel(log.InfoLevel)
	p.Stderr = cmdLogger.WriterLevel(log.ErrorLevel)
	return p.Run()
}

func git(ctx context.Context, repoDir string, args ...string) error {
	return run(ctx, repoDir, nil, "/usr/bin/git", args...)
}

func cleanup(deployKey, knownHosts string) {
	if knownHosts != "" {
		log.Infof("Removing known_hosts file %s", knownHosts)
		if err := os.Remove(knownHosts); err != nil {
			log.WithError(err).Error("Failed cleaning up known_hosts key")
		}
	}

	if deployKey != "" {
		log.Infof("Removing deploy key file %s", deployKey)
		if err := os.Remove(deployKey); err != nil {
			log.WithError(err).Error("Failed cleaning up deploy key")
		}
	}
}

// getGithubHostKeys fetches the github host keys from the github metadata
// service. The metadata is fetched over HTTPS, and so we have built-on
// protection against MitM attacks while fetching the expected host keys.
func getGithubHostKeys(ctx context.Context) ([]ssh.PublicKey, error) {
	metadata, err := github.GetMetadata(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed fetching github metadata")
	}

	// extract the host keys
	githubHostKeys, err := metadata.HostKeys()
	if err != nil {
		return nil, trace.Wrap(err, "failed fetching github hostKeys")
	}

	return githubHostKeys, nil
}
