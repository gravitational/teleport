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

package genericoidc

import (
	"context"
	"errors"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestGetIDTokenFromEnvironment(t *testing.T) {
	t.Parallel()

	fakeGetEnv := func(expectKey, env string) envGetter {
		return func(key string) string {
			if key == expectKey {
				return env
			}

			return ""
		}
	}

	const keyName = "OIDC_TEST"

	tests := []struct {
		name        string
		getEnv      envGetter
		wantString  string
		assertError require.ErrorAssertionFunc
	}{
		{
			name:        "success",
			getEnv:      fakeGetEnv(keyName, "foobarbizz"),
			wantString:  "foobarbizz",
			assertError: require.NoError,
		},
		{
			name:   "unset",
			getEnv: fakeGetEnv(keyName, ""),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewIDTokenSource(tt.getEnv, DefaultCommandRunner)
			str, err := v.GetIDTokenFromEnvironment(keyName)
			require.Equal(t, tt.wantString, str)
			tt.assertError(t, err)
		})
	}
}

func TestGetIDTokenFromCommand(t *testing.T) {
	t.Parallel()

	// fakeRunner produces command runners that simulate a delay and return a
	// set value; fast when run under synctest.
	fakeRunner := func(delay time.Duration, ret func() ([]byte, error)) commandRunner {
		return func(ctx context.Context, command ...string) ([]byte, error) {
			select {
			case <-time.After(delay):
				return ret()
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	tests := []struct {
		name        string
		runner      commandRunner
		timeout     time.Duration
		wantString  string
		assertError require.ErrorAssertionFunc
	}{
		{
			name:    "success",
			timeout: time.Minute,
			runner: fakeRunner(time.Second, func() ([]byte, error) {
				return []byte("foo"), nil
			}),
			wantString:  "foo",
			assertError: require.NoError,
		},
		{
			name:    "error",
			timeout: time.Minute,
			runner: fakeRunner(time.Second, func() ([]byte, error) {
				return nil, errors.New("oops")
			}),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "oops")
			},
		},
		{
			name:    "timeout",
			timeout: time.Minute,
			runner: fakeRunner(time.Hour, func() ([]byte, error) {
				return []byte("impossible"), nil
			}),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, context.DeadlineExceeded)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				v := NewIDTokenSource(os.Getenv, tt.runner)

				str, err := v.GetIDTokenFromCommand(t.Context(), tt.timeout, "foo")
				require.Equal(t, tt.wantString, str)
				tt.assertError(t, err)
			})
		})
	}
}
