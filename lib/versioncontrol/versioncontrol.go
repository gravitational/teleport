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
// the teleport.Version variable.
// NOTE: this isn't equivalent to "canonicalization" which makes equivalent version strings
// comparable via `==`. version strings returned by this function should still only be compared
// via `semver.Compare`.
func Normalize(v string) string {
	if semver.IsValid(v) {
		return v
	}

	if n := fmt.Sprintf("v%s", v); semver.IsValid(n) {
		return n
	}

	return v
}

// Visitor is a helper for aggregating information about observed versions. Useful for
// getting latest/oldest version observed during iteration/pagination. Zero value omits
// prereleases.
type Visitor struct {
	PermitPrerelease bool
	latest           string
	oldest           string
}

// Visit processes the supplied version string. If ok is false, the string was
// ignored due to being invalid, or because if was a prerelease if the visitor
// is configured to ignore those.
func (v *Visitor) Visit(s string) (ok bool) {
	if !semver.IsValid(s) {
		return false
	}

	if !v.PermitPrerelease && semver.Prerelease(s) != "" {
		return false
	}

	if v.latest == "" || semver.Compare(v.latest, s) == -1 {
		v.latest = s
	}

	if v.oldest == "" || semver.Compare(v.oldest, s) == 1 {
		v.oldest = s
	}

	return true
}

// Latest gets the most recent version string from among those observed.
func (v *Visitor) Latest() string {
	return v.latest
}

// Oldest gets the oldest version string from among those observed.
func (v *Visitor) Oldest() string {
	return v.oldest
}
