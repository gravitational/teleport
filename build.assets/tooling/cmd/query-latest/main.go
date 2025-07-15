/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Command query-latest returns the highest semver release for a versionSpec
// query-latest ignores drafts and pre-releases. If the latest release for
// a version is in teleport-private, the version tag will be prefixed with
// "private-".
//
// For example:
//
//	query-latest v8.1.5     -> v8.1.5
//	query-latest v8.1.3     -> error, no matching release (this is a tag, but not a release)
//	query-latest v8.0.0-rc3 -> error, no matching release (this is a pre-release, in github and in semver)
//	query-latest v7.0       -> v7.0.2
//	query-latest v5         -> v5.2.4
//	query-latest v17        -> private-v17.5.2 (latest v17 release is in teleport-private)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/mod/semver" //nolint:depguard // Usage precedes the x/mod/semver rule.

	"github.com/gravitational/teleport/build.assets/tooling/lib/github"
)

var errNoResults = errors.New("no releases matched")

func main() {
	versionSpec, err := parseFlags()
	if err != nil {
		log.Fatalf("Failed to parse flags: %v.", err)
	}

	var gh github.GitHub
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		gh = github.NewGitHubWithToken(context.Background(), token)
	} else {
		gh = github.NewGitHub()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tag, err := getLatest(ctx, versionSpec, gh)
	if err != nil {
		log.Fatalf("Query failed: %v.", err)
	}

	fmt.Println(tag)
}

func parseFlags() (string, error) {
	flag.Parse()
	if flag.NArg() == 0 {
		return "", trace.BadParameter("missing argument: vX.X")
	} else if flag.NArg() > 1 {
		return "", trace.BadParameter("unexpected positional arguments: %q", flag.Args()[1:])
	}

	versionSpec := flag.Args()[0]

	return versionSpec, nil
}

func getLatest(ctx context.Context, versionSpec string, gh github.GitHub) (string, error) {
	publicTag, err := getLatestForRepo(ctx, versionSpec, gh, "teleport")
	if err != nil {
		return "", trace.Wrap(err)
	}

	privateTag, err := getLatestForRepo(ctx, versionSpec, gh, "teleport-private")
	// Ignore errNoResults errors from teleport-private as it is quite possible that
	// there have been no private releases on a branch or in a particular minor
	// release set.
	if err != nil && !errors.Is(err, errNoResults) {
		return "", trace.Wrap(err)
	}
	if err == nil && semver.Compare(publicTag, privateTag) < 0 {
		return "private-" + privateTag, nil
	}

	return publicTag, nil
}

func getLatestForRepo(ctx context.Context, versionSpec string, gh github.GitHub, repo string) (string, error) {
	releases, err := gh.ListReleases(ctx, "gravitational", repo)
	if err != nil {
		return "", trace.Wrap(err, "couldn't list releases for repo %q", repo)
	}
	// The repos we check for releases all have releases. If we do not get anything
	// back, this is an error as something must have gone wrong.
	if len(releases) == 0 {
		return "", trace.NotFound("failed to find any releases on GitHub for repo %q", repo)
	}

	// filter drafts and prereleases, which shouldn't be tracked by latest docker images
	var tags []string
	for _, r := range releases {
		if r.GetDraft() {
			continue
		}
		if r.GetPrerelease() {
			continue
		}
		tag := r.GetTagName()
		if semver.Prerelease(tag) != "" {
			continue
		}
		tags = append(tags, tag)
	}

	semver.Sort(tags)

	// semver.Sort is ascending, so we loop in reverse
	for i := len(tags) - 1; i >= 0; i-- {
		tag := tags[i]
		if strings.HasPrefix(tag, versionSpec) {
			return tag, nil
		}
	}

	return "", trace.Wrap(errNoResults, "version %q, repo %q", versionSpec, repo)
}
