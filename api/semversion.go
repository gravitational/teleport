// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import "github.com/coreos/go-semver/semver"

// SemVer returns the version of Teleport in use as a [semver.Version].
func SemVer() *semver.Version {
	return &semver.Version{
		Major:      VersionMajor,
		Minor:      VersionMinor,
		Patch:      VersionPatch,
		PreRelease: VersionPreRelease,
		Metadata:   VersionMetadata,
	}
}

// SemVersion contains the same value you'd get by calling [SemVer].
//
// Deprecated: call [SemVer] instead.
var SemVersion = SemVer()

// TODO(espadolini) DELETE IN v19
