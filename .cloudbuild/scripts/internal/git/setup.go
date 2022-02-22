package git

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
// will be configured to use that. The config
func Configure(repoDir string, deployKey []byte) (cfg *Config, err error) {
	var identity string
	var hostsFile string

	// The deploy key we're using is too sensitive to just hope that every
	// exit path will clean it up on failure, so let's gust register a
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
		// configure SSH known_hosts for github.com
		hostsFile, err = configureKnownHosts()
		if err != nil {
			return nil, trace.Wrap(err, "failed configuring known hosts")
		}

		// GCB clones via https, and our deploy key won't work with that, so
		// force git to access the remote over ssh with a rewrite rule
		log.Info("Adding remote url rewrite rule")
		err = git(repoDir, "config", `url.git@github.com:.insteadOf`, "https://github.com/")
		if err != nil {
			return nil, trace.Wrap(err, "failed configuring url rewrite rule")
		}

		// set up the identity for the deploy key
		identity, err = writeKey(deployKey)
		if err != nil {
			return nil, trace.Wrap(err, "failed configuring SSH identity")
		}

		// finally, force git to use our custom SSH setup when accessing the remote
		log.Infof("Configuring git ssh command")
		err = git(repoDir, "config", "core.sshCommand",
			fmt.Sprintf("ssh -i %s -o UserKnownHostsFile=%s", identity, hostsFile))
		if err != nil {
			return nil, trace.Wrap(err, "failed configuring git to use deploy key")
		}
	}

	return &Config{identity: identity, repoDir: repoDir, knownHosts: hostsFile}, nil
}

// Do runs `git args...` in the configured repository
func (cfg *Config) Do(args ...string) error {
	return git(cfg.repoDir, args...)
}

// Close cleans up the repository, including deleting the deployment key (if any)
func (cfg *Config) Close() error {
	cleanup(cfg.identity, cfg.knownHosts)
	return nil
}

func run(workingDir string, env []string, cmd string, args ...string) error {
	p := exec.Command(cmd, args...)
	if len(env) != 0 {
		p.Env = append(os.Environ(), env...)
	}
	p.Dir = workingDir
	p.Stdout = os.Stdout
	p.Stderr = os.Stderr
	return p.Run()
}

func git(repoDir string, args ...string) error {
	return run(repoDir, nil, "/usr/bin/git", args...)
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
