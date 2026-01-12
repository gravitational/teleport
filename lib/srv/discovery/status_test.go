/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package discovery

import (
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

func TestTruncateErrorMessage(t *testing.T) {
	for _, tt := range []struct {
		name     string
		in       discoveryconfig.Status
		expected *string
	}{
		{
			name:     "nil error message",
			in:       discoveryconfig.Status{},
			expected: nil,
		},
		{
			name:     "small error messages are not changed",
			in:       discoveryconfig.Status{ErrorMessage: stringPointer("small error message")},
			expected: stringPointer("small error message"),
		},
		{
			name:     "large error messages are truncated",
			in:       discoveryconfig.Status{ErrorMessage: stringPointer(strings.Repeat("A", 1024*100+1))},
			expected: stringPointer(strings.Repeat("A", 1024*100)),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateErrorMessage(tt.in)
			require.Equal(t, tt.expected, got)
		})
	}
}

type mockInstance struct {
	syncTime       *timestamppb.Timestamp
	discoveryGroup string
}

func (m *mockInstance) GetSyncTime() *timestamppb.Timestamp {
	return m.syncTime
}

func (m *mockInstance) GetDiscoveryGroup() string {
	return m.discoveryGroup
}

func TestMergeExistingInstances(t *testing.T) {
	clock := clockwork.NewFakeClock()
	pollInterval := 10 * time.Minute
	s := &Server{
		Config: &Config{
			clock:          clock,
			PollInterval:   pollInterval,
			DiscoveryGroup: "group-1",
		},
	}

	now := clock.Now()
	tooOld := now.Add(-3 * pollInterval)
	recent := now.Add(-pollInterval)

	tests := []struct {
		name           string
		oldInstances   map[string]*mockInstance
		freshInstances map[string]*mockInstance
		expected       map[string]*mockInstance
	}{
		{
			name: "skip instances from the same discovery group",
			oldInstances: map[string]*mockInstance{
				"inst-1": {
					syncTime:       timestamppb.New(recent),
					discoveryGroup: "group-1",
				},
			},
			freshInstances: map[string]*mockInstance{},
			expected:       map[string]*mockInstance{},
		},
		{
			name: "skip expired instances",
			oldInstances: map[string]*mockInstance{
				"inst-2": {
					syncTime:       timestamppb.New(tooOld),
					discoveryGroup: "group-2",
				},
			},
			freshInstances: map[string]*mockInstance{},
			expected:       map[string]*mockInstance{},
		},
		{
			name: "merge missing instances",
			oldInstances: map[string]*mockInstance{
				"inst-3": {
					syncTime:       timestamppb.New(recent),
					discoveryGroup: "group-2",
				},
			},
			freshInstances: map[string]*mockInstance{},
			expected: map[string]*mockInstance{
				"inst-3": {
					syncTime:       timestamppb.New(recent),
					discoveryGroup: "group-2",
				},
			},
		},
		{
			name: "do not overwrite fresh instances",
			oldInstances: map[string]*mockInstance{
				"inst-4": {
					syncTime:       timestamppb.New(recent),
					discoveryGroup: "group-2",
				},
			},
			freshInstances: map[string]*mockInstance{
				"inst-4": {
					syncTime:       timestamppb.New(now),
					discoveryGroup: "group-1",
				},
			},
			expected: map[string]*mockInstance{
				"inst-4": {
					syncTime:       timestamppb.New(now),
					discoveryGroup: "group-1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workingCopy := maps.Clone(tt.freshInstances)
			mergeExistingInstances(s, tt.oldInstances, workingCopy)
			require.Equal(t, tt.expected, workingCopy)
		})
	}
}
