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

// Command version-check validates that a tag is not a prerelease
// or that it is the latest version ever. version-check exits non-zero
// if tag fails this check. This allows us to avoid updating "latest"
// packages or tags when publishing releases for older branches.
package main

import (
	"context"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/mod/semver"

	"github.com/gravitational/teleport/build.assets/tooling/lib/github"
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
		err = checkLatest(ctx, tag, github.NewGitHub())

	case "prerelease":
		// Assert that the supplied tag is a valid semver version with no
		// prerelease or build-metadata components
		err = checkIsBareRelease(tag)

	case "valid":
		// Assert that the supplied tag is a valid semver version string.
		err = checkValidSemver(tag)

	default:
		log.Fatalf("invalid check: %v", check)
	}

	if err != nil {
		log.Fatalf("Check failed: %v.", err)
	}
}

func parseFlags() (string, string, error) {
	tag := flag.String("tag", "", "tag to validate")
	check := flag.String("check", "", "check to run [latest, prerelease, valid]")
	flag.Parse()

	if *tag == "" {
		return "", "", trace.BadParameter("tag missing")
	}
	if *check == "" {
		return "", "", trace.BadParameter("check missing")
	}
	switch *check {
	case "latest", "prerelease", "valid":
	default:
		return "", "", trace.BadParameter("invalid check: %v", *check)
	}

	return *tag, *check, nil
}

func checkLatest(ctx context.Context, tag string, gh github.GitHub) error {
	releases, err := gh.ListReleases(ctx, "gravitational", "teleport")
	if err != nil {
		return trace.Wrap(err)
	}
	if len(releases) == 0 {
		return trace.BadParameter("failed to find any releases on GitHub")
	}

	var tags []string
	for _, r := range releases {
		if r.GetDraft() {
			continue
		}
		// Because pre-releases are not published to apt, we do not want to
		// consider them when making apt publishing decisions.
		// see: https://github.com/gravitational/teleport/issues/10800
		if semver.Prerelease(r.GetTagName()) != "" {
			continue
		}
		tags = append(tags, r.GetTagName())
	}

	semver.Sort(tags)
	latest := tags[len(tags)-1]
	if semver.Compare(tag, latest) <= 0 {
		return trace.BadParameter("found newer version of release, not releasing. Latest release: %v, tag: %v", latest, tag)
	}

	return nil
}

// checkValidSemver returns an error if the supplied string is not a valid
// Semver identifier
func checkValidSemver(tag string) error {
	if !semver.IsValid(tag) {
		return trace.BadParameter("version is invalid semver: %v", tag)
	}
	return nil
}

// checkIsBareRelease returns nil if the supplied tag is a valid semver
// version string without pre-release or build-metadata components. Returns
// an error if any of these conditions is not met.
func checkIsBareRelease(tag string) error {
	if err := checkValidSemver(tag); err != nil {
		return trace.Wrap(err)
	}

	if semver.Prerelease(tag) != "" { // https://semver.org/#spec-item-9
		return trace.BadParameter("version is pre-release: %v", tag)
	}
	if strings.Contains(tag, "+") { // https://semver.org/#spec-item-10
		return trace.BadParameter("version contains build metadata: %v", tag)
	}
	return nil
}
