package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
)

func main() {
	err := innerMain()
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}
}

func innerMain() error {
	var workspacePath string
	flag.StringVar(&workspacePath, "workspace", "", "The place to checkout the teleport repo")

	var targetBranch string
	flag.StringVar(&targetBranch, "target", "", "The branch being targeted by the PR")

	flag.Parse()

	if workspacePath == "" {
		return fmt.Errorf("must supply a path")
	}

	err := initSSH()
	if err != nil {
		return fmt.Errorf("failed configuring SSH: %w", err)
	}
	defer cleanupSSH()

	err = initWorkspace(workspacePath, targetBranch, "/workspace")
	if err != nil {
		return fmt.Errorf("failed initialising workspace: %w", err)
	}

	return nil
}

func initSSH() error {
	sshConfigDir := path.Join("/", "root", ".ssh")
	err := os.MkdirAll(sshConfigDir, 0700)
	if err != nil {
		return fmt.Errorf("failed creating ssh config dir: %w", err)
	}

	fmt.Printf(">>> Configuring known hosts\n")
	knownHostsFile := path.Join(sshConfigDir, "known_hosts")
	script := fmt.Sprintf("ssh-keyscan -H github.com > %q 2>/dev/null", knownHostsFile)
	err = run("/bin/bash", "-c", script)
	if err != nil {
		return fmt.Errorf("failed adding github.com to known hosts: %w", err)
	}

	err = os.Chmod(knownHostsFile, 0600)
	if err != nil {
		return fmt.Errorf("failed setting known_hosts permissions: %w", err)
	}

	fmt.Printf(">>> Configuring deployment SSH keys\n")

	webassetsKeyFile := path.Join(sshConfigDir, "webassets-e")
	err = os.WriteFile(
		webassetsKeyFile,
		[]byte(os.Getenv("WEBASSETS_DEPLOYMENT_KEY")),
		0600)
	if err != nil {
		return fmt.Errorf("failed writing deployment SSH key: %w", err)
	}

	sshConfigPath := path.Join(sshConfigDir, "config")
	configFile, err := os.OpenFile(sshConfigPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed opening ssh config file %q: %w", sshConfigPath, err)
	}
	defer configFile.Close()

	for _, keyFile := range []string{webassetsKeyFile} {
		_, err := fmt.Fprintf(configFile, "IdentityFile %s\n", keyFile)
		if err != nil {
			return fmt.Errorf("failed adding deployment SSH key %q: %w", keyFile, err)
		}
	}

	return nil
}

func cleanupSSH() {
	os.RemoveAll("/root/.ssh")
}

func initWorkspace(teleportPath string, branch string, workspaceDir string) error {
	git := func(args ...string) error {
		return git(teleportPath, args...)
	}

	fmt.Printf(">>> Creating teleport workspace dir %q\n", teleportPath)
	err := os.MkdirAll(teleportPath, 0744)
	if err != nil {
		return err
	}

	fmt.Printf(">>> Initialising repo...\n")
	err = git("init")
	if err != nil {
		return err
	}

	err = git("remote", "add", "origin", "https://github.com/gravitational/teleport")
	if err != nil {
		return fmt.Errorf("failed adding origin: %w", err)
	}

	fmt.Printf(">>> Fetching OSS Teleport (%s)...\n", branch)

	err = git("fetch", "origin", "+refs/heads/"+branch)
	if err != nil {
		return fmt.Errorf("failed fetching branch %q: %w", branch, err)
	}

	err = git("checkout", branch)
	if err != nil {
		return fmt.Errorf("failed checking out branch %q: %w", branch, err)
	}

	fmt.Printf(">>> Checked out branch teleport:%s at ", branch)
	git("rev-parse", "HEAD")

	// This may fail in pre-4.3 Teleport versions that don't use the webassets
	// submodule - remove this error check if porting to old branches.
	fmt.Printf(">>> Fetching Webassets...\n")
	err = git("submodule", "update", "--init", "--recursive", "webassets")
	if err != nil {
		return fmt.Errorf("failed fetching webassets: %w", err)
	}

	fmt.Printf(">>> Copying workspace into teleport/e\n")
	submoduleDir := path.Join(teleportPath, "e")
	err = os.RemoveAll(submoduleDir)
	if err != nil {
		return fmt.Errorf("failed removing submodule dir %q: %w", submoduleDir, err)
	}

	err = run("cp", "-r", workspaceDir, submoduleDir)
	if err != nil {
		return fmt.Errorf("failed copying %q -> %q: %w", submoduleDir, workspaceDir, err)
	}

	return nil
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
