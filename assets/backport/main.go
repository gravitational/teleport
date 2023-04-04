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
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/teleport/assets/backport/github"
	"gopkg.in/yaml.v2"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	backportBranches, prNumber, owner, repo, err := parseFlags()
	if err != nil {
		log.Fatal(err)
	}

	// Getting the Github token from ~/.config/gh/hosts.yml
	token, err := getGithubToken()
	if err != nil {
		log.Fatal(err)
	}

	clt, err := github.New(ctx, &github.Config{
		Token:        token,
		Repository:   repo,
		Organization: owner,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, targetBranch := range backportBranches {
		// New branches will be in the format:
		// auto-backport/[pull request number]-to-[target branch name].
		newBranchName, err := clt.Backport(ctx, targetBranch, prNumber)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Backported commits to branch %s.\n", newBranchName)

		// Create the pull request.
		if err = clt.CreatePullRequest(ctx, targetBranch, newBranchName, prNumber); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Pull request created for branch %s.\n", newBranchName)
	}
	fmt.Println("Backporting complete.")
}

// Config is used to unmarshal the Github
// CLI config.
type Config struct {
	// Host is the host name of the
	// server.
	Host Host `yaml:"github.com"`
}

type Host struct {
	// Token is Github token.
	Token string `yaml:"oauth_token"`
}

// githubConfigPath is the default config path
// (relative to user's home directory) for the
// Github CLI tool.
const githubConfigPath = ".config/gh/hosts.yml"

// getGithubToken gets the Github auth token from
// the Github CLI config.
func getGithubToken() (string, error) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	ghConfigPath := filepath.Join(dirname, githubConfigPath)
	yamlFile, err := os.ReadFile(ghConfigPath)
	if err != nil {
		return "", trace.Wrap(err)
	}

	config := new(Config)
	if err = yaml.Unmarshal(yamlFile, config); err != nil {
		return "", trace.Wrap(err)
	}
	if config.Host.Token == "" {
		return "", trace.BadParameter("missing GitHub token.")
	}
	return config.Host.Token, nil
}

// parseFlags parses flags and sets
func parseFlags() ([]string, int, string, string, error) {
	var (
		to    = flag.String("to", "", "List of comma-separated branch names to backport to.\n Ex: branch/v6,branch/v7\n")
		pr    = flag.Int("pr", 0, "Pull request with changes to backport.")
		owner = flag.String("owner", "gravitational", "Name of the repository's owner.")
		repo  = flag.String("repo", "teleport", "Name of the repository to open up pull requests in.")
	)
	flag.Parse()
	if *to == "" {
		return nil, 0, "", "", trace.BadParameter("must supply branches to backport to.")
	}
	if *pr == 0 {
		return nil, 0, "", "", trace.BadParameter("much supply pull request with changes to backport.")
	}
	// Parse branches to backport to.
	backportBranches, err := parseBranches(*to)
	if err != nil {
		return nil, 0, "", "", trace.Wrap(err)
	}
	return backportBranches, *pr, *owner, *repo, nil
}

// parseBranches parses a string of comma separated branch
// names into a string slice.
func parseBranches(branchesInput string) ([]string, error) {
	var backportBranches []string
	branches := strings.Split(branchesInput, ",")
	for _, branch := range branches {
		if branch == "" {
			return nil, trace.BadParameter("received an empty branch name.")
		}
		backportBranches = append(backportBranches, strings.TrimSpace(branch))
	}
	return backportBranches, nil
}
