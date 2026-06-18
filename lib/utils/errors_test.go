/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package utils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeTimeoutError mirrors http2httpError: net.Error with Timeout() ==
// true and no Is method. Temporary() is required by the net.Error
// interface (deprecated but not removed as of Go 1.25).
type fakeTimeoutError struct{ msg string }

func (e *fakeTimeoutError) Error() string   { return e.msg }
func (e *fakeTimeoutError) Timeout() bool   { return true }
func (e *fakeTimeoutError) Temporary() bool { return true }

// fakeNonTimeoutError is a net.Error with Timeout() == false.
type fakeNonTimeoutError struct{ msg string }

func (e *fakeNonTimeoutError) Error() string   { return e.msg }
func (e *fakeNonTimeoutError) Timeout() bool   { return false }
func (e *fakeNonTimeoutError) Temporary() bool { return true }

func TestCanExplainNetworkError_TimeoutFallthrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		wantOK  bool
		wantSub string
	}{
		{
			name:    "http2 response header timeout shape",
			err:     &fakeTimeoutError{msg: "http2: timeout awaiting response headers"},
			wantOK:  true,
			wantSub: "Context Deadline Exceeded",
		},
		{
			name:   "net error without Timeout returns no explanation",
			err:    &fakeNonTimeoutError{msg: "some non-timeout net error"},
			wantOK: false,
		},
		{
			name:   "plain error returns no explanation",
			err:    errors.New("not a net.Error"),
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg, ok := CanExplainNetworkError(tt.err)
			require.Equal(t, tt.wantOK, ok)
			if tt.wantSub != "" {
				require.Contains(t, msg, tt.wantSub)
			}
		})
	}
}
