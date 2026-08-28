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

package versioncontrol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecurityPatchAlts(t *testing.T) {
	tts := []struct {
		desc string
		a, b Target
		alt  bool
	}{
		{
			desc: "basic alt case",
			a:    NewTarget("v1.2.3", SecurityPatch(true), SecurityPatchAlts("v1.2.2", "v1.2.1")),
			b:    NewTarget("v1.2.1"),
			alt:  true,
		},
		{
			desc: "minimal alt case",
			a:    NewTarget("v1.2.3", SecurityPatchAlts("v1.2.1")),
			b:    NewTarget("v1.2.1"),
			alt:  true,
		},
		{
			desc: "trivial non-alt case",
			a:    NewTarget("v1.2.3"),
			b:    NewTarget("v1.2.1"),
			alt:  false,
		},
		{
			desc: "non-matching non-alt case case",
			a:    NewTarget("v1.2.3", SecurityPatchAlts("v1.2.2")),
			b:    NewTarget("v1.2.1"),
			alt:  false,
		},
	}

	for _, tt := range tts {
		// check alt status is expected
		require.Equal(t, tt.alt, tt.a.SecurityPatchAltOf(tt.b), "desc=%q", tt.desc)
		// check alt status is bidirectional
		require.Equal(t, tt.alt, tt.b.SecurityPatchAltOf(tt.a), "desc=%q", tt.desc)
	}
}

func TestVisitorBasics(t *testing.T) {
	tts := []struct {
		versions         []string
		newest           string
		oldest           string
		notNewerThan     string
		permitPrerelease bool
		desc             string
	}{
		{
			versions: []string{
				"v1.2.3",
				"v2.3.4-alpha.1",
			},
			newest: "v1.2.3",
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
			newest: "v3.5.7",
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
			newest:           "v3.4.5-alpha.1",
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
			newest:           "v3.4.4",
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
			newest:           "v3.4.5-alpha.1",
			oldest:           "v0.1.2",
			permitPrerelease: true,
			desc:             "prerelease on (mixed)",
		},
		{
			versions: []string{
				"v1.2.3",
				"v3.4.5",
				"v1.1.1",
				"v2.2.2",
				"v2.1.0",
			},
			notNewerThan: "v2.1.1",
			newest:       "v2.1.0",
			oldest:       "v1.1.1",
			desc:         "not newer than",
		},
	}

	for _, tt := range tts {
		visitor := Visitor{
			PermitPrerelease: tt.permitPrerelease,
			NotNewerThan:     NewTarget(tt.notNewerThan),
		}

		for _, v := range tt.versions {
			visitor.Visit(Target{LabelVersion: v})
		}

		require.Equal(t, tt.newest, visitor.Newest().Version(), tt.desc)
		require.Equal(t, tt.oldest, visitor.Oldest().Version(), tt.desc)
	}
}

func TestVisitorRelative(t *testing.T) {
	tts := []struct {
		current       Target
		targets       []Target
		nextMajor     Target
		newestCurrent Target
		newestSec     Target
		notNewerThan  Target
		desc          string
	}{
		{
			current: NewTarget("v1.2.3"),
			targets: []Target{
				NewTarget("v1.3.5", SecurityPatch(true)),
				NewTarget("v2.3.4"),
				NewTarget("v2", SecurityPatch(true)),
				NewTarget("v0.1", SecurityPatch(true)),
				NewTarget("v2.4.2"),
				NewTarget("v1.4.4"),
				NewTarget("v3.4.5"),
			},
			nextMajor:     NewTarget("v2.4.2"),
			newestCurrent: NewTarget("v1.4.4"),
			newestSec:     NewTarget("v1.3.5", SecurityPatch(true)),
			desc:          "broad test case",
		},
		{
			targets: []Target{
				NewTarget("v1.3.5", SecurityPatch(true)),
				NewTarget("v2.3.4"),
				NewTarget("v2", SecurityPatch(true)),
				NewTarget("v0.1", SecurityPatch(true)),
				NewTarget("v2.4.2"),
				NewTarget("v1.4.4"),
			},
			desc: "no current target specified",
		},
		{
			current: NewTarget("v1.2.3"),
			targets: []Target{
				NewTarget("v1.1"),
				NewTarget("v1", SecurityPatch(true)),
				NewTarget("v0.1"),
			},
			newestCurrent: NewTarget("v1.1"),
			newestSec:     NewTarget("v1", SecurityPatch(true)),
			desc:          "older targets",
		},
		{
			current: NewTarget("v3.5.6"),
			targets: []Target{
				NewTarget("v1.2.3"),
				NewTarget("v2.3.4", SecurityPatch(true)),
				NewTarget("v0.1.2"),
			},
			desc: "too old",
		},
		{
			current: NewTarget("v1.2.3"),
			targets: []Target{
				NewTarget("v3.4.5"),
				NewTarget("v3", SecurityPatch(true)),
				NewTarget("v12.13.14"),
			},
			desc: "too new",
		},
		{
			current: NewTarget("v9"),
			targets: []Target{
				NewTarget("v10.0.1"),
				NewTarget("v10", SecurityPatch(true)),
				NewTarget("v9.0.1"),
				NewTarget("v9", SecurityPatch(true)),
			},
			nextMajor:     NewTarget("v10.0.1"),
			newestCurrent: NewTarget("v9.0.1"),
			newestSec:     NewTarget("v9", SecurityPatch(true)),
			desc:          "carry the one",
		},
		{
			current: NewTarget("v1.5.9"),
			targets: []Target{
				NewTarget("v2.2.2"),
				NewTarget("v1.2.3"),
				NewTarget("v2.4.8", SecurityPatch(true)),
				NewTarget("v1", SecurityPatch(true)),
			},
			notNewerThan:  NewTarget("v1.3.5"),
			newestCurrent: NewTarget("v1.2.3"),
			newestSec:     NewTarget("v1", SecurityPatch(true)),
			desc:          "not newer than",
		},
	}

	for _, tt := range tts {
		visitor := Visitor{
			Current:      tt.current,
			NotNewerThan: tt.notNewerThan,
		}

		for _, target := range tt.targets {
			visitor.Visit(target)
		}

		require.Equal(t, tt.nextMajor, visitor.NextMajor(), tt.desc)
		require.Equal(t, tt.newestCurrent, visitor.NewestCurrent(), tt.desc)
		require.Equal(t, tt.newestSec, visitor.NewestSecurityPatch(), tt.desc)
	}
}
