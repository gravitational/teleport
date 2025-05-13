/*
Copyright 2023 Gravitational, Inc.

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
