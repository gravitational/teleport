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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVisitor(t *testing.T) {
	tts := []struct {
		versions         []string
		latest           string
		oldest           string
		permitPrerelease bool
		desc             string
	}{
		{
			versions: []string{
				"v1.2.3",
				"v2.3.4-alpha.1",
			},
			latest: "v1.2.3",
			oldest: "v1.2.3",
			desc:   "one stable release",
		},
		{
			versions: []string{
				"v1.2.3",
				"v2.3.4",
				"v2.2.2",
				"v3.5.7",
				"invalid",
				"v0.0.1-alpha.2",
			},
			latest: "v3.5.7",
			oldest: "v1.2.3",
			desc:   "mixed releases",
		},
		{
			versions: []string{
				"invalid",
				"12356",
				"127.0.0.1:8080",
			},
			desc: "all invalid",
		},
		{
			versions: []string{
				"v3.4.5-alpha.1",
				"v3.4.4",
				"v0.1.2-alpha.2",
				"v0.1.11",
			},
			latest:           "v3.4.5-alpha.1",
			oldest:           "v0.1.2-alpha.2",
			permitPrerelease: true,
			desc:             "prerelease on",
		},
		{
			versions: []string{
				"v3.4.5-alpha.1",
				"v3.4.4",
				"v0.1.2-alpha.2",
				"v0.1.11",
			},
			latest:           "v3.4.4",
			oldest:           "v0.1.11",
			permitPrerelease: false,
			desc:             "prerelease off",
		},
		{
			versions: []string{
				"v3.4.5-alpha.1",
				"v3.4.4",
				"v0.1.12-alpha.2",
				"v0.1.2",
			},
			latest:           "v3.4.5-alpha.1",
			oldest:           "v0.1.2",
			permitPrerelease: true,
			desc:             "prerelease on (mixed)",
		},
	}

	for _, tt := range tts {
		visitor := Visitor{
			PermitPrerelease: tt.permitPrerelease,
		}

		for _, v := range tt.versions {
			visitor.Visit(v)
		}

		require.Equal(t, tt.latest, visitor.Latest(), tt.desc)
		require.Equal(t, tt.oldest, visitor.Oldest(), tt.desc)
	}
}
