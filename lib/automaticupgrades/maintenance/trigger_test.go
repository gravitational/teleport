/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package maintenance

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// checkTraceError is a test helper that converts trace.IsXXXError into a require.ErrorAssertionFunc
func checkTraceError(check func(error) bool) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, i ...interface{}) {
		require.True(t, check(err), i...)
	}
}

func TestFailoverTrigger_CanStart(t *testing.T) {
	t.Parallel()

	// Test setup
	ctx := context.Background()
	tests := []struct {
		name         string
		triggers     []Trigger
		expectResult bool
		expectErr    require.ErrorAssertionFunc
	}{
		{
			name:         "nil",
			triggers:     nil,
			expectResult: false,
			expectErr:    checkTraceError(trace.IsNotFound),
		},
		{
			name:         "empty",
			triggers:     []Trigger{},
			expectResult: false,
			expectErr:    checkTraceError(trace.IsNotFound),
		},
		{
			name: "first trigger success firing",
			triggers: []Trigger{
				StaticTrigger{canStart: true},
				StaticTrigger{canStart: false},
			},
			expectResult: true,
			expectErr:    require.NoError,
		},
		{
			name: "first trigger success not firing",
			triggers: []Trigger{
				StaticTrigger{canStart: false},
				StaticTrigger{canStart: true},
			},
			expectResult: false,
			expectErr:    require.NoError,
		},
		{
			name: "first trigger failure",
			triggers: []Trigger{
				StaticTrigger{err: trace.LimitExceeded("got rate-limited")},
				StaticTrigger{canStart: true},
			},
			expectResult: false,
			expectErr:    checkTraceError(trace.IsLimitExceeded),
		},
		{
			name: "first trigger skipped, second getter success",
			triggers: []Trigger{
				StaticTrigger{err: trace.NotImplemented("proxy does not seem to implement RFD-184")},
				StaticTrigger{canStart: true},
			},
			expectResult: true,
			expectErr:    require.NoError,
		},
		{
			name: "first trigger skipped, second getter failure",
			triggers: []Trigger{
				StaticTrigger{err: trace.NotImplemented("proxy does not seem to implement RFD-184")},
				StaticTrigger{err: trace.LimitExceeded("got rate-limited")},
			},
			expectResult: false,
			expectErr:    checkTraceError(trace.IsLimitExceeded),
		},
		{
			name: "first trigger skipped, second getter skipped",
			triggers: []Trigger{
				StaticTrigger{err: trace.NotImplemented("proxy does not seem to implement RFD-184")},
				StaticTrigger{err: trace.NotImplemented("proxy does not seem to implement RFD-184")},
			},
			expectResult: false,
			expectErr:    checkTraceError(trace.IsNotFound),
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				// Test execution
				trigger := FailoverTrigger(tt.triggers)
				result, err := trigger.CanStart(ctx, nil)
				require.Equal(t, tt.expectResult, result)
				tt.expectErr(t, err)
			},
		)
	}
}

func TestFailoverTrigger_Name(t *testing.T) {
	tests := []struct {
		name         string
		triggers     []Trigger
		expectResult string
	}{
		{
			name:         "nil",
			triggers:     nil,
			expectResult: "",
		},
		{
			name:         "empty",
			triggers:     []Trigger{},
			expectResult: "",
		},
		{
			name: "one trigger",
			triggers: []Trigger{
				StaticTrigger{name: "proxy"},
			},
			expectResult: "proxy",
		},
		{
			name: "two triggers",
			triggers: []Trigger{
				StaticTrigger{name: "proxy"},
				StaticTrigger{name: "version-server"},
			},
			expectResult: "proxy, failover version-server",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				// Test execution
				trigger := FailoverTrigger(tt.triggers)
				result := trigger.Name()
				require.Equal(t, tt.expectResult, result)
			},
		)
	}
}
