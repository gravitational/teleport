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

package version

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

const (
	semverLow         = "v11.3.2"
	semverMid         = "v11.5.4"
	semverHigh        = "v12.2.1"
	invalidSemverHigh = "12.2.1"
)

func TestValidVersionChange(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		current string
		next    string
		want    bool
	}{
		{
			name:    "upgrade",
			current: semverMid,
			next:    semverHigh,
			want:    true,
		},
		{
			name:    "same version",
			current: semverMid,
			next:    semverMid,
			want:    false,
		},
		{
			name:    "unknown current version",
			current: "",
			next:    semverMid,
			want:    true,
		},
		{
			name:    "non-semver current version",
			current: semverMid,
			next:    invalidSemverHigh,
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				require.Equal(t, tt.want, ValidVersionChange(ctx, tt.current, tt.next))
			},
		)
	}
}

// checkTraceError is a test helper that converts trace.IsXXXError into a require.ErrorAssertionFunc
func checkTraceError(check func(error) bool) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, i ...interface{}) {
		require.True(t, check(err), i...)
	}
}

func TestFailoverGetter_GetVersion(t *testing.T) {
	t.Parallel()

	// Test setup
	ctx := context.Background()
	tests := []struct {
		name         string
		getters      []Getter
		expectResult string
		expectErr    require.ErrorAssertionFunc
	}{
		{
			name:         "nil",
			getters:      nil,
			expectResult: "",
			expectErr:    checkTraceError(trace.IsNotFound),
		},
		{
			name:         "empty",
			getters:      []Getter{},
			expectResult: "",
			expectErr:    checkTraceError(trace.IsNotFound),
		},
		{
			name: "first getter success",
			getters: []Getter{
				StaticGetter{version: semverMid},
				StaticGetter{version: semverHigh},
			},
			expectResult: semverMid,
			expectErr:    require.NoError,
		},
		{
			name: "first getter failure",
			getters: []Getter{
				StaticGetter{err: trace.LimitExceeded("got rate-limited")},
				StaticGetter{version: semverHigh},
			},
			expectResult: "",
			expectErr:    checkTraceError(trace.IsLimitExceeded),
		},
		{
			name: "first getter skipped, second getter success",
			getters: []Getter{
				StaticGetter{err: trace.NotImplemented("proxy does not seem to implement RFD-184")},
				StaticGetter{version: semverHigh},
			},
			expectResult: semverHigh,
			expectErr:    require.NoError,
		},
		{
			name: "first getter skipped, second getter failure",
			getters: []Getter{
				StaticGetter{err: trace.NotImplemented("proxy does not seem to implement RFD-184")},
				StaticGetter{err: trace.LimitExceeded("got rate-limited")},
			},
			expectResult: "",
			expectErr:    checkTraceError(trace.IsLimitExceeded),
		},
		{
			name: "first getter skipped, second getter skipped",
			getters: []Getter{
				StaticGetter{err: trace.NotImplemented("proxy does not seem to implement RFD-184")},
				StaticGetter{err: trace.NotImplemented("proxy does not seem to implement RFD-184")},
			},
			expectResult: "",
			expectErr:    checkTraceError(trace.IsNotFound),
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				// Test execution
				getter := FailoverGetter(tt.getters)
				result, err := getter.GetVersion(ctx)
				require.Equal(t, tt.expectResult, result)
				tt.expectErr(t, err)
			},
		)
	}
}
