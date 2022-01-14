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

// Command version-check validates that a tag is not a prerelease
// or that it is the latest version ever. version-check exits non-zero
// if tag fails this check. This allows us to avoid updating "latest"
// packages or tags when publishing releases for older branches.
package main

import (
	"context"
	"flag"
	"log"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/gravitational/trace"

	go_github "github.com/google/go-github/v41/github"
)

func main() {
	tag, check, err := parseFlags()
	if err != nil {
		log.Fatalf("Failed to parse flags: %v.", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch check {
	case "latest":
		err = checkLatest(ctx, tag, newGitHub())
	case "prerelease":
		err = checkPrerelease(tag)
	default:
		log.Fatalf("invalid check: %v", check)
	}

	if err != nil {
		log.Fatalf("Check failed: %v.", err)
	}
}

func parseFlags() (string, string, error) {
	tag := flag.String("tag", "", "tag to validate")
	check := flag.String("check", "", "check to run [latest, prerelease]")
	flag.Parse()

	if *tag == "" {
		return "", "", trace.BadParameter("tag missing")
	}
	if *check == "" {
		return "", "", trace.BadParameter("check missing")
	}
	switch *check {
	case "latest", "prerelease":
	default:
		return "", "", trace.BadParameter("invalid check: %v", *check)
	}

	return *tag, *check, nil
}

func checkLatest(ctx context.Context, tag string, gh github) error {
	releases, err := gh.ListReleases(ctx, "gravitational", "teleport")
	if err != nil {
		return trace.Wrap(err)
	}
	sort.SliceStable(releases, func(i int, j int) bool {
		return releases[i] > releases[j]
	})
	if len(releases) == 0 {
		return trace.BadParameter("failed to find any releases on GitHub")
	}

	if semver.Compare(tag, releases[0]) <= 0 {
		return trace.BadParameter("found newer version of release, not releasing. Latest release: %v, tag: %v", releases[0], tag)
	}

	return nil
}

func checkPrerelease(tag string) error {
	if semver.Prerelease(tag) != "" { // https://semver.org/#spec-item-9
		return trace.BadParameter("version is pre-release: %v", tag)
	}
	if strings.Contains(tag, "+") { // https://semver.org/#spec-item-10
		return trace.BadParameter("version contains build metadata: %v", tag)
	}
	return nil
}

type github interface {
	ListReleases(ctx context.Context, organization string, repository string) ([]string, error)
}

type ghClient struct {
	client *go_github.Client
}

func newGitHub() *ghClient {
	return &ghClient{
		client: go_github.NewClient(nil),
	}
}

func (c *ghClient) ListReleases(ctx context.Context, organization string, repository string) ([]string, error) {
	var releases []string

	opt := &go_github.ListOptions{
		Page:    0,
		PerPage: 100,
	}
	for n := 0; n < 100; n++ {
		page, resp, err := c.client.Repositories.ListReleases(ctx,
			organization,
			repository,
			opt)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, p := range page {
			releases = append(releases, p.GetTagName())
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return releases, nil
}
