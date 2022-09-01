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

package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gravitational/teleport"
	vc "github.com/gravitational/teleport/lib/versioncontrol"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"golang.org/x/mod/semver"
)

// NOTE: when making modifications to package, make sure to run tests with
// `TEST_GITHUB_API=yes`. this will enable some additional tests that are not
// run as part of normal CI.

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentVersionControl,
})

// LatestStable gets the most recent "stable" (not pre-release) version.
// NOTE: this may return a version *older* than the current teleport version
// if the current teleport version is a newer prerelease.
func LatestStable() (string, error) {
	return latestStable(Iterator{}, vc.Normalize(teleport.Version))
}

// latestStable is the business logic of LatestStable, broken out for testing purposes.
func latestStable(iter Iterator, current string) (string, error) {
	if !semver.IsValid(current) {
		return "", trace.BadParameter("cannot get latest stable, invalid semver: %q", current)
	}
	// we only care about newer releaseas, so set halting point to 'current'
	iter.halt = current

	// set up visitor that will default to 'current' if nothing newer is observed.
	var visitor vc.Visitor
	for iter.Next() {
		for _, r := range iter.Page() {
			visitor.Visit(r.Version)
		}
	}
	return visitor.Latest(), trace.Wrap(iter.Error())
}

// defaultHaltingPoint represents the default cutoff for the iterator. this value
// is expanded/truncated to `<major>.<minor>` so e.g. `v1` would halt iteration on the
// first `v1.0.X` release observed, and `v1.2.3` would halt iteration on the first
// `v1.2.X` release observed.
const defaultHaltingPoint = "v8"

const defaultPageSize = 100

// Release represents a github release.
type Release struct {
	// Version is the semver version from the git tag of the release.
	Version string

	// TODO(fspmarshall): decide on a scheme for embedding additional tags
	// in our git releases (e.g. security-patch=yes, etc).
}

// release is the representation of a release returned
// by the github API.
type release struct {
	TagName string `json:"tag_name"`
}

// getter loads a page of releases. we override the standard page loading logic
// via this type in order to avoid issues with rate-limiting of the unauthenticated
// releases API.
type getter func(n, size int) ([]release, error)

// getPage loads the specified page from the unauthenticated github releases API
// using the provided http client.
func getPage(clt http.Client, n, size int) ([]release, error) {
	if n == 0 {
		return nil, trace.BadParameter("unspecified page number for teleport/releases (this is a bug)")
	}
	if size == 0 {
		return nil, trace.BadParameter("unspecified page size for teleport/releases (this is a bug)")
	}

	// get page from the github api. see https://docs.github.com/rest/releases/releases for details.
	res, err := clt.Get(fmt.Sprintf("https://api.github.com/repos/gravitational/teleport/releases?page=%d&per_page=%d", n, size))
	if err != nil {
		return nil, trace.Errorf("failed to get teleport/releases: %v", err)
	}

	body, err := io.ReadAll(res.Body)
	res.Body.Close()

	if res.StatusCode > 299 {
		// attempt to extract pretty error message
		var gherr ghError
		if json.Unmarshal(body, &gherr) == nil {
			if gherr.Message != "" {
				return nil, trace.Errorf("teleport/releases request failed with message: %s (%d)", gherr.Message, res.StatusCode)
			}
		}
		// fallback to status code and quoted body
		return nil, trace.Errorf("teleport/releases request failed with code=%d, body=%q", res.StatusCode, body)
	}

	// handle error from rsp body read
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return parsePage(body)
}

// deserialization helper for github api errors
type ghError struct {
	Message string `json:"message"`
}

func parsePage(page []byte) ([]release, error) {
	var releases []release
	if err := json.Unmarshal(page, &releases); err != nil {
		return nil, trace.Errorf("failed to unmarshal releases page: %v", err)
	}
	return releases, nil
}

// Iterator allows lazy consumption of github release pages. Not safe for
// concurrent use. Always check for err after iteration, even if one or more
// pages were loaded successfully. If a release being added concurrently with
// iteration may cause duplicate releases to be observed.
type Iterator struct {
	getPage getter
	n       int
	size    int
	page    []Release
	halt    string
	done    bool
	err     error
}

func (i *Iterator) setDefaults() {
	if i.getPage == nil {
		var clt http.Client
		i.getPage = func(n, size int) ([]release, error) {
			return getPage(clt, n, size)
		}
	}

	if i.size == 0 {
		i.size = defaultPageSize
	}

	if i.halt == "" {
		i.halt = defaultHaltingPoint
	}
}

// Next attempts to load the next page.
func (i *Iterator) Next() bool {
	i.setDefaults()
	if i.done {
		return false
	}
	i.n++
	var page []release
	page, i.err = i.getPage(i.n, i.size)
	if len(page) < i.size {
		// check unfiltered page size for halt condition
		i.done = true
	}
	i.page = make([]Release, 0, len(page))
	for _, r := range page {
		if !semver.IsValid(r.TagName) {
			log.Debugf("Skipping non-semver release tag: %q\n", r.TagName)
			continue
		}
		i.page = append(i.page, Release{
			Version: r.TagName,
		})

		// only match `<major>.<minor>` when finding halt point. theoretically
		// unnecessary, but this feels a bit less brittle than halting on a specific
		// tag, and is more consistent than using a gt/lt rule, since we occasionally
		// release patches for very old versions.
		if semver.MajorMinor(r.TagName) == semver.MajorMinor(i.halt) {
			i.done = true
			break
		}
	}
	return i.err == nil && len(page) != 0
}

// Page loads the current page. Must not be called until after Next() has been
// called. Subsequent calls between calls to Next() return the same value.
func (i *Iterator) Page() []Release {
	return i.page
}

// Error checks if an error occurred during iteration.
func (i *Iterator) Error() error {
	return i.err
}
