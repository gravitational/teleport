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
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

// NOTE: the contents of this file might be moving to the 'api' package in the future.

// isValidLabel checks if a string is a valid target label or value. this function is exposed
// as two separate helpers below in order to simplfiy things if we start having different
// criterea for keys and values in the future.
var isValidLabel = regexp.MustCompile(`^[a-z0-9\.\-\/]+$`).MatchString

// isValidLabelVal is the same as isValidLabel except that it also permits '|' which is interpreted
// as a list delmeter for some labels.
var isValidLabelVal = regexp.MustCompile(`^[a-z0-9\.\-\/|]+$`).MatchString

// IsValidTargetKey checks if a string is a valid installation target key.
func IsValidTargetKey(key string) bool {
	return isValidLabel(key)
}

// IsValidTargetVal checks if a string is a valid installtion target value.
func IsValidTargetVal(val string) bool {
	return isValidLabelVal(val)
}

// LabelSecurityPatch indicates that a release is a security patch when set to 'yes'.
const LabelSecurityPatch = "security-patch"

// LabelSecurityPatchAlts lists alternative versions that provide the same security fixes.
const LabelSecurityPatchAlts = "security-patch-alts"

// LabelVersion is the only required label for an installation target and must be
// valid go-style semver.
const LabelVersion = "version"

// TargetOption is a functional option for setting additional target fields during construction.
type TargetOption func(*targetOptions)

type targetOptions struct {
	secPatch bool
	secAlts  []string
}

// SecurityPatch sets the security-patch=yes label if true.
func SecurityPatch(sec bool) TargetOption {
	return func(opts *targetOptions) {
		opts.secPatch = sec
	}
}

// SecurityPatchAlts appends version strings to the security-patch-alts label.
func SecurityPatchAlts(alts ...string) TargetOption {
	return func(opts *targetOptions) {
		opts.secAlts = append(opts.secAlts, alts...)
	}
}

// Target is a description of an available installation target. A given "release"
// (e.g. v1.2.3) may consist of one or more targets depending on how intallation
// targets are being modeled (e.g. TUF creates a separate target for each
// installation package for a given version, whereas the github releases scraper
// simply creates one target per version). Unknown keypairs should be ignored, and
// invalid values for expected labels should cause the target as a whole to be ignored.
// If the Target is being used in a manner that would cause it to be written to the backend,
// and therefore potentially be used later on by a newer version of teleport, then invalid
// keypairs should be preserved since they may have known meaning to newer versions.
type Target map[string]string

// NewTarget creates a new target with the specified version and options.
func NewTarget(version string, opts ...TargetOption) Target {
	var options targetOptions
	for _, opt := range opts {
		opt(&options)
	}
	target := Target{
		LabelVersion: version,
	}
	if options.secPatch {
		target[LabelSecurityPatch] = "yes"
	}

	if len(options.secAlts) != 0 {
		target[LabelSecurityPatchAlts] = strings.Join(options.secAlts, "|")
	}

	return target
}

// Ok checks if the target is valid. The only requirement for a target to be valid
// is for it to have a version field containing valid go-style semver. This must be
// checked prior to use of the target (this method also works as a nil check).
func (t Target) Ok() bool {
	return semver.IsValid(t.Version())
}

// Version gets the version of this installation target. Note that the returned value is not
// necessarily canonicalized (i.e. two equivalent version strings may not compare as equal).
func (t Target) Version() string {
	return t[LabelVersion]
}

// Major gets the major version of this target (e.g. if version=v1.2.3 then Major() returns v1).
func (t Target) Major() string {
	return semver.Major(t.Version())
}

// NextMajor gets the next major version string (e.g. if version=v2.3.4 then NextMajor() returns v3).
func (t Target) NextMajor() string {
	m := t.Major()
	if len(m) < 2 {
		return ""
	}
	n, err := strconv.ParseUint(m[1:], 10, 64)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("v%d", n+1)
}

// SecurityPatch checks for the special label 'security-patch=yes'.
func (t Target) SecurityPatch() bool {
	return t[LabelSecurityPatch] == "yes"
}

// SecurityPatchAltOf performs a bidirectional check to see if this target and another
// are security patch alternates (i.e. wether or not they provide the same security fix).
func (t Target) SecurityPatchAltOf(other Target) bool {
	if !t.Ok() || !other.Ok() {
		return false
	}

	var alt bool

	t.iterSecAlts(func(v string) {
		if semver.Compare(v, other.Version()) == 0 {
			alt = true
		}
	})

	other.iterSecAlts(func(v string) {
		if semver.Compare(v, t.Version()) == 0 {
			alt = true
		}
	})

	return alt
}

// iterSecAlts is a helper for iterating the valide values of the
// security-patch-alts label.
func (t Target) iterSecAlts(fn func(v string)) {
	for _, alt := range strings.Split(t[LabelSecurityPatchAlts], "|") {
		alt = strings.TrimSpace(alt)
		if !semver.IsValid(alt) {
			continue
		}

		fn(alt)
	}
}

// Prerelease checks if this target represents a prerelease installation target
// (e.g. v1.2.3-alpha.1).
func (t Target) Prerelease() bool {
	return semver.Prerelease(t.Version()) != ""
}

// NewerThan returns true if this target has a well-defined "newer" relationship to
// other. Returns false if other is older, equal, or incomparable (e.g. if one or both
// targets have invalid versions).
func (t Target) NewerThan(other Target) bool {
	if !t.Ok() || !other.Ok() {
		return false
	}

	return semver.Compare(t.Version(), other.Version()) == 1
}

// OlderThan returns true if this target has a well-defined "newer" relationship to
// other. Returns false if other is older, equal, or incomparable (e.g. if one or both
// targets have invalid versions).
func (t Target) OlderThan(other Target) bool {
	if !t.Ok() || !other.Ok() {
		return false
	}

	return semver.Compare(t.Version(), other.Version()) == -1
}

// VersionEquals returns true if this target has a well-defined equivalence relationship to the
// version of other. Returns false if the versons are not equal, or if they are incomparable
// (e.g. if one or both targets have invalid versions).
func (t Target) VersionEquals(other Target) bool {
	if !t.Ok() || !other.Ok() {
		return false
	}

	return semver.Compare(t.Version(), other.Version()) == 0
}
