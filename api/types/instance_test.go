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

package types

import (
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
)

func TestInstanceFilter(t *testing.T) {
	iis := []struct {
		id          string
		version     string
		services    []SystemRole
		upgrader    string
		updateGroup string
	}{
		{
			id:          "a1",
			version:     "v1.2.3",
			services:    []SystemRole{RoleAuth},
			updateGroup: "default",
		},
		{
			id:          "a2",
			version:     "v2.3.4",
			services:    []SystemRole{RoleAuth, RoleNode},
			upgrader:    "kube",
			updateGroup: "default",
		},
		{
			id:          "p1",
			version:     "v1.2.1",
			services:    []SystemRole{RoleProxy},
			updateGroup: "foobar",
		},
		{
			id:       "p2",
			version:  "v2.3.1",
			services: []SystemRole{RoleProxy, RoleNode},
			upgrader: "unit",
		},
	}

	// set up group of test instances
	var instances []Instance
	for _, ii := range iis {
		var UpdaterInfo *UpdaterV2Info
		if ii.updateGroup != "" {
			UpdaterInfo = &UpdaterV2Info{
				UpdateGroup: ii.updateGroup,
			}
		}

		ins, err := NewInstance(ii.id, InstanceSpecV1{
			Version:          ii.version,
			Services:         ii.services,
			ExternalUpgrader: ii.upgrader,
			UpdaterInfo:      UpdaterInfo,
		})

		require.NoError(t, err, "id=%s", ii.id)
		instances = append(instances, ins)
	}

	// set up test scenarios
	tts := []struct {
		desc    string
		filter  InstanceFilter
		matches []string
	}{
		{
			desc:   "match-all",
			filter: InstanceFilter{},
			matches: []string{
				"a1",
				"a2",
				"p1",
				"p2",
			},
		},
		{
			desc: "match-proxies",
			filter: InstanceFilter{
				Services: []SystemRole{
					RoleProxy,
				},
			},
			matches: []string{
				"p1",
				"p2",
			},
		},
		{
			desc: "match-old",
			filter: InstanceFilter{
				OlderThanVersion: "v2",
			},
			matches: []string{
				"a1",
				"p1",
			},
		},
		{
			desc: "match-new",
			filter: InstanceFilter{
				NewerThanVersion: "v2",
			},
			matches: []string{
				"a2",
				"p2",
			},
		},
		{
			desc: "match-version-range",
			filter: InstanceFilter{
				NewerThanVersion: "v1.2.2",
				OlderThanVersion: "v2.3.3",
			},
			matches: []string{
				"a1",
				"p2",
			},
		},
		{
			desc: "match-kube-upgrader",
			filter: InstanceFilter{
				ExternalUpgrader: "kube",
			},
			matches: []string{
				"a2",
			},
		},
		{
			desc: "match-no-upgrader",
			filter: InstanceFilter{
				NoExtUpgrader: true,
			},
			matches: []string{
				"a1",
				"p1",
			},
		},
		{
			desc: "match-update-group",
			filter: InstanceFilter{
				UpdateGroup: "default",
			},
			matches: []string{
				"a1",
				"a2",
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			var matches []string
			for _, ins := range instances {
				if tt.filter.Match(ins) {
					matches = append(matches, ins.GetName())
				}
			}

			require.Equal(t, tt.matches, matches)
		})
	}
}

// TestVersionShorthand verifies our ability to decode go-style semver with the more
// opinionated semver library.
func TestVersionShorthand(t *testing.T) {
	tts := []struct {
		version string
		expect  string
		invalid bool
	}{
		{
			version: "v1",
			expect:  "1.0.0",
		},
		{
			version: "1.1",
			expect:  "1.1.0",
		},
		{
			version: "1",
			expect:  "1.0.0",
		},
		{
			version: "v1.2.3",
			expect:  "1.2.3",
		},
		{
			version: "v1.2",
			expect:  "1.2.0",
		},
		{
			version: "1.2.3-alpha.1",
			expect:  "1.2.3-alpha.1",
		},
		{
			version: "v1.2.3-alpha.1",
			expect:  "1.2.3-alpha.1",
		},
		{
			version: "1.2.3+amd64",
			expect:  "1.2.3+amd64",
		},
		{
			version: "1.2.3-alpha.1+amd64",
			expect:  "1.2.3-alpha.1+amd64",
		},
		{
			version: "v1.2.3-alpha.1+amd64",
			expect:  "1.2.3-alpha.1+amd64",
		},
		{
			version: "1v2.3",
			invalid: true,
		},
		{
			version: "",
			invalid: true,
		},
		{
			version: ".",
			invalid: true,
		},
		{
			version: "v",
			invalid: true,
		},
		{
			version: "vv",
			invalid: true,
		},
		{
			version: "v1-alpha.1",
			invalid: true,
		},
		{
			version: "v1.2-alpha.1",
			invalid: true,
		},
	}

	for _, tt := range tts {
		vr, ok := parseVersionRelaxed(tt.version)
		if tt.invalid {
			require.False(t, ok, "tt=%+v", tt)
			continue
		}

		require.True(t, ok, "tt=%+v", tt)

		vs, err := semver.NewVersion(tt.expect)
		require.NoError(t, err, "tt=%+v", tt)

		require.Equal(t, tt.expect, vr.String(), "tt=%+v", tt)
		require.True(t, vr.Equal(*vs), "tt=%+v", tt)
	}
}

func TestInstanceControlLogExpiry(t *testing.T) {
	const ttl = time.Minute
	now := time.Now()
	instance, err := NewInstance("test-instance", InstanceSpecV1{
		LastSeen: now,
	})
	require.NoError(t, err)

	instance.AppendControlLog(
		InstanceControlLogEntry{
			Type: "foo",
			Time: now,
			TTL:  ttl,
		},
		InstanceControlLogEntry{
			Type: "bar",
			Time: now.Add(-ttl / 2),
			TTL:  ttl,
		},
		InstanceControlLogEntry{
			Type: "bin",
			Time: now.Add(-ttl * 2),
			TTL:  ttl,
		},
		InstanceControlLogEntry{
			Type: "baz",
			Time: now,
			TTL:  time.Hour,
		},
	)

	require.Len(t, instance.GetControlLog(), 4)

	instance.SyncLogAndResourceExpiry(ttl)

	require.Len(t, instance.GetControlLog(), 3)
	require.Equal(t, now.Add(time.Hour).UTC(), instance.Expiry())

	instance.SetLastSeen(now.Add(ttl))

	instance.SyncLogAndResourceExpiry(ttl)

	require.Len(t, instance.GetControlLog(), 2)
	require.Equal(t, now.Add(time.Hour).UTC(), instance.Expiry())

	instance.AppendControlLog(
		InstanceControlLogEntry{
			Type: "long-lived",
			Time: now,
			TTL:  time.Hour * 2,
		},
	)

	instance.SyncLogAndResourceExpiry(ttl)

	require.Len(t, instance.GetControlLog(), 3)
	require.Equal(t, now.Add(time.Hour*2).UTC(), instance.Expiry())
}
