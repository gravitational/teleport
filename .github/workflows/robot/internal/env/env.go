/*
Copyright 2021 Gravitational, Inc.

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

package env

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/gravitational/trace"
)

// Environment is the execution environment the workflow is running in.
type Environment struct {
	// Organization is the GitHub organization (gravitational).
	Organization string

	// Repository is the GitHub repository (teleport).
	Repository string

	// Number is the PR number.
	Number int

	// Author is the author of the PR.
	Author string

	// UnsafeHead is the name of the branch the workflow is running in.
	//
	// UnsafeHead can be attacker controlled and should not be used in any
	// security sensitive context. For example, don't use it when crafting a URL
	// to send a request to or an access decision. See the following link for
	// more details:
	//
	// https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections
	UnsafeHead string

	// UnsafeBase is the name of the base branch the user is trying to merge the
	// PR into. For example: "master" or "branch/v8".
	//
	// UnsafeBase can be attacker controlled and should not be used in any
	// security sensitive context. For example, don't use it when crafting a URL
	// to send a request to or an access decision. See the following link for
	// more details:
	//
	// https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections
	UnsafeBase string
}

// New returns a new execution environment for the workflow.
func New() (*Environment, error) {
	event, err := readEvent()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the event does not have a action associated with it (for example a cron
	// run), read in organization/repository from the environment.
	if event.Action == "" {
		organization, repository, err := readEnvironment()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Environment{
			Organization: organization,
			Repository:   repository,
		}, nil
	}

	return &Environment{
		Organization: event.Repository.Owner.Login,
		Repository:   event.Repository.Name,
		Number:       event.PullRequest.Number,
		Author:       event.PullRequest.User.Login,
		UnsafeHead:   event.PullRequest.UnsafeHead.UnsafeRef,
		UnsafeBase:   event.PullRequest.UnsafeBase.UnsafeRef,
	}, nil
}

func readEvent() (*Event, error) {
	f, err := os.Open(os.Getenv(githubEventPath))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	var event Event
	if err := json.NewDecoder(f).Decode(&event); err != nil {
		return nil, trace.Wrap(err)
	}

	return &event, nil
}

func readEnvironment() (string, string, error) {
	repository := os.Getenv(githubRepository)
	if repository == "" {
		return "", "", trace.BadParameter("%v environment variable missing", githubRepository)
	}
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return "", "", trace.BadParameter("failed to parse organization and/or repository")
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", trace.BadParameter("invalid organization and/or repository")
	}
	return parts[0], parts[1], nil
}

const (
	// githubEventPath is an environment variable that contains a path to the
	// GitHub event for a workflow run.
	githubEventPath = "GITHUB_EVENT_PATH"

	// githubRepository is an environment variable that contains the organization
	// and repository name.
	githubRepository = "GITHUB_REPOSITORY"
)
