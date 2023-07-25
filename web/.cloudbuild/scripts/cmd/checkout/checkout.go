// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/gravitational/trace"
)

func main() {
	err := innerMain()
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}
}

type cliArgs struct {
	workspace string
}

func innerMain() error {
	args, err := parseCLI()
	if err != nil {
		return trace.Wrap(err)
	}

	// We need to inject deployment keys into the user's SSH config. Note that we
	// assume we're running as root.
	err = initSSH()
	if err != nil {
		return trace.Wrap(err)
	}
	defer cleanupSSH()

	log.Print("Updating submodules...")
	err = git(args.workspace, "submodule", "update", "--init", "--recursive")
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func parseCLI() (cliArgs, error) {
	args := cliArgs{}

	flag.StringVar(&args.workspace, "w", "", "Path to the workspace to act on")

	flag.Parse()

	if args.workspace == "" {
		return cliArgs{}, trace.Errorf("workspace path must be set")
	}

	var err error
	args.workspace, err = filepath.Abs(args.workspace)
	if err != nil {
		return cliArgs{}, trace.Wrap(err, "Unable to resole absolute path to workspace")
	}

	return args, nil
}

func initSSH() error {
	sshConfigDir := path.Join("/", "root", ".ssh")
	err := os.MkdirAll(sshConfigDir, 0700)
	if err != nil {
		return trace.Wrap(err, "failed creating ssh config dir")
	}

	log.Printf("Configuring known hosts in %s", sshConfigDir)
	knownHostsFile := path.Join(sshConfigDir, "known_hosts")
	script := fmt.Sprintf("ssh-keyscan -H github.com > %q 2>/dev/null", knownHostsFile)
	err = run("/bin/bash", "-c", script)
	if err != nil {
		return trace.Wrap(err, "failed adding github.com to known hosts")
	}
	err = os.Chmod(knownHostsFile, 0600)
	if err != nil {
		return trace.Wrap(err, "failed setting known_hosts permissions")
	}

	log.Print("Configuring deployment SSH keys")

	key := os.Getenv("WEBAPPS_E_DEPLOYMENT_KEY")
	if key == "" {
		return trace.Errorf("webapps.e deployment key not in environment")
	}

	webappsKeyFile := path.Join(sshConfigDir, "webapps.e")
	log.Printf("Writing webassets deployment key to %s", webappsKeyFile)
	err = writeFile(
		webappsKeyFile,
		[]byte(key),
		0600)
	if err != nil {
		return trace.Wrap(err, "failed writing deployment SSH key")
	}

	sshConfigPath := path.Join(sshConfigDir, "config")
	configFile, err := os.OpenFile(sshConfigPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return trace.Wrap(err, "failed opening ssh config file %q", sshConfigPath)
	}
	defer configFile.Close()

	for _, keyFile := range []string{webappsKeyFile} {
		_, err := fmt.Fprintf(configFile, "IdentityFile %s\n", keyFile)
		if err != nil {
			return trace.Wrap(err, "failed adding deployment SSH key %q", keyFile)
		}
	}

	return nil
}

func cleanupSSH() {
	os.RemoveAll("/root/.ssh")
}

func git(repoDir string, args ...string) error {
	return runInDir(repoDir, "/usr/bin/git", args...)
}

func run(cmd string, args ...string) error {
	return runInDir("", cmd, args...)
}

func runInDir(dir string, cmd string, args ...string) error {
	p := exec.Command(cmd, args...)
	p.Dir = dir
	p.Stdout = os.Stdout
	p.Stderr = os.Stderr
	return p.Run()
}

// writeFile is a backport of os.WriteFile() so that we can run this script in
// versions of Go older than 1.16
func writeFile(name string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	return err
}
