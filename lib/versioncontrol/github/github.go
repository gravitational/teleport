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

package github

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/mod/semver"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	vc "github.com/gravitational/teleport/lib/versioncontrol"
)

// NOTE: when making modifications to package, make sure to run tests with
// `TEST_GITHUB_API=yes`. this will enable some additional tests that are not
// run as part of normal CI.

var logger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentVersionControl)

// Visit uses the supplied visitor to aggregate release info from the github releases api.
func Visit(visitor *vc.Visitor) error {
	return visit(Iterator{}, visitor)
}

// visit is the business logic of Visit, broken out for testing purposes.
func visit(iter Iterator, visitor *vc.Visitor) error {
	if !visitor.Current.Ok() {
		return trace.BadParameter("cannot scrape github releases, invalid 'current' target: %+v", visitor.Current)
	}

	// we only care about newer releaseas, so set halting point to the version of 'current'
	iter.halt = visitor.Current.Version()

	for iter.Next() {
		for _, target := range iter.Page() {
			visitor.Visit(target)
		}
	}
	return trace.Wrap(iter.Error())
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

	// OtherLabels are the labels extracted from the release notes.
	OtherLabels map[string]string

	// TODO(fspmarshall): get rid of this type in favor of a common attribute-based
	// release representation based in lib/versioncontrol.
}

// release is the representation of a release returned
// by the github API.
type release struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
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
	page    []vc.Target
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
	i.page = make([]vc.Target, 0, len(page))
	for _, r := range page {
		if !semver.IsValid(r.TagName) {
			logger.DebugContext(context.Background(), "Skipping non-semver release tag", "tag_name", r.TagName)
			continue
		}
		labels := parseReleaseNoteLabels(r.Body)
		labels[vc.LabelVersion] = r.TagName
		i.page = append(i.page, labels)

		// only match `<major>.<minor>` when finding halt point. theoretically
		// unnecessary, but this feels a bit less brittle than halting on a specific
		// tag, and is more consistent than using a gt/lt rule, since we occasionally
		// release patches for very old versions.
		if semver.MajorMinor(r.TagName) == semver.MajorMinor(i.halt) {
			// set 'done' so that this is the last page we end up processing
			i.done = true
		}
	}
	return i.err == nil && len(page) != 0
}

// Page loads the current page. Must not be called until after Next() has been
// called. Subsequent calls between calls to Next() return the same value.
func (i *Iterator) Page() []vc.Target {
	return i.page
}

// Error checks if an error occurred during iteration.
func (i *Iterator) Error() error {
	return i.err
}

// parseReleaseNoteLabels attempts to extract labels from github release notes.
// Invalid values are skipped/ignored in order to ensure that future extensions of
// the label format can be reasonably backwards-compatible. Labels are encoded
// in the form '\nlabels:<key>=<val>[,<key>=<val>]'. Characters are normalize to
// lowercase and spaces between keypairs are stripped. See TestLabelParse for
// examples.
func parseReleaseNoteLabels(notes string) map[string]string {
	const labelPrefix = "labels:"

	labels := make(map[string]string)

	scanner := bufio.NewScanner(strings.NewReader(notes))

	for scanner.Scan() {
		l := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if !strings.HasPrefix(l, labelPrefix) {
			continue
		}
		l = strings.TrimPrefix(l, labelPrefix)
		for _, kv := range strings.Split(l, ",") {
			if !strings.Contains(kv, "=") {
				logger.DebugContext(context.Background(), "Skipping invalid release label keypair", "label", kv)
				continue
			}

			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				logger.DebugContext(context.Background(), "Skipping invalid release label keypair", "label", kv)
				continue
			}

			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])

			if !vc.IsValidTargetKey(key) || !vc.IsValidTargetVal(val) {
				logger.DebugContext(context.Background(), "Skipping invalid release label keypair", "label", kv)
				// NOTE: we are skipping invalid keypairs for github release scraping
				// because github releases are using a generally simplistic release representation.
				// The TUF implementation will not skip invalid keypairs, preferring to
				// preserve them in the backend in order to ensure that backend representations
				// are forward-compatible with future versions auth version.
				continue
			}

			labels[key] = val
		}
	}

	return labels
}
