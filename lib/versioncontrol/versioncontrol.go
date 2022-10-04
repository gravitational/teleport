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

package versioncontrol

import (
	"fmt"

	"golang.org/x/mod/semver"
)

// Normalize attaches the expected `v` prefix to a version string if the supplied
// version is currently invalid, and attaching the prefix makes it valid. Useful for normalizing
// the teleport.Version variable. Note that this package generally treats targets and version strings
// as immutable, so normalization is never applied automatically. It is the responsibility of the
// consumers of this package to apply normalization when and where immutability is known to not
// be required (e.g. the 'teleport.Version' can and should always be normalized).
//
// NOTE: this isn't equivalent to "canonicalization" which makes equivalent version strings
// comparable via `==`. version strings returned by this function should still only be compared
// via `semver.Compare`, or via the comparison methods on the Target type.
func Normalize(v string) string {
	if semver.IsValid(v) {
		return v
	}

	if n := fmt.Sprintf("v%s", v); semver.IsValid(n) {
		return n
	}

	return v
}

// Visitor is a helper for aggregating information about observed targets. Useful for
// getting newest/oldest version observed during iteration/pagination. Zero value omits
// prereleases.
//
// TODO(fspmarshall): Rework this to handle scenarios where multiple targets have the same
// version and add collection for .newest current version", "latest next major version",
// and .newest security patch" (relative to an optional "current version" param).
type Visitor struct {
	// PermitPrerelease configures whether or not the visitor will process/yield targets
	// with semver prerelease versions.
	PermitPrerelease bool

	// NotNewerThan is an optional target represented a constraint for the *newest* version
	// that we care about. Targets newer than NotNewerThan are ignored if it is supplied.
	NotNewerThan Target

	// Current is an optional target representing the current installation. If a valid
	// target is supplied, then the Next* family of targets are selected relative to it.
	Current Target

	newest         Target
	oldest         Target
	nextMajor      Target
	newestCurrent  Target
	newestSecPatch Target
}

// Visit processes the supplied target. If ok is false, the target was
// ignored due to being invalid, or because it was a prerelease if the visitor
// is configured to ignore those.
func (v *Visitor) Visit(t Target) (ok bool) {
	if !t.Ok() {
		return false
	}

	if !v.PermitPrerelease && t.Prerelease() {
		return false
	}

	if v.NotNewerThan.Ok() && t.NewerThan(v.NotNewerThan) {
		return false
	}

	if !v.newest.Ok() || t.NewerThan(v.newest) {
		v.newest = t
	}

	if !v.oldest.Ok() || t.OlderThan(v.oldest) {
		v.oldest = t
	}

	if v.Current.Ok() {
		switch t.Major() {
		case v.Current.Major():
			if !v.newestCurrent.Ok() || t.NewerThan(v.newestCurrent) {
				v.newestCurrent = t
			}
			if t.SecurityPatch() {
				if !v.newestSecPatch.Ok() || t.NewerThan(v.newestSecPatch) {
					v.newestSecPatch = t
				}
			}
		case v.Current.NextMajor():
			if !v.nextMajor.Ok() || t.NewerThan(v.nextMajor) {
				v.nextMajor = t
			}
		}
	}

	return true
}

// Newest gets the most recent version string from among those observed.
func (v *Visitor) Newest() Target {
	return v.newest
}

// Oldest gets the oldest version string from among those observed.
func (v *Visitor) Oldest() Target {
	return v.oldest
}

// NextMajor gets the newest target from the next major version (nil if Current was not
// supplied or no matches were found).
func (v *Visitor) NextMajor() Target {
	return v.nextMajor
}

// NewestCurrent gets the newest target from the current major version (nil if Current was
// not supplied or no matches were found). Note that this target may not actually be newer
// than than the current target.
func (v *Visitor) NewestCurrent() Target {
	return v.newestCurrent
}

// NewestSecurityPatch gets the newest target from the current major version which is a
// security patch (nil if Current was not supplied or if no matches were found). Note that
// this target may not actually be newer than the current target.
func (v *Visitor) NewestSecurityPatch() Target {
	return v.newestSecPatch
}
