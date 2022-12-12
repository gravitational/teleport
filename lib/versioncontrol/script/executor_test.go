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

package script

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	vc "github.com/gravitational/teleport/api/versioncontrol"
)

func TestExecutor(t *testing.T) {
	t.Parallel()

	tts := []struct {
		params  types.ExecScript
		success bool
		output  string
	}{
		{
			params: types.ExecScript{
				Type:   "trivial-case",
				ID:     1,
				Script: `echo "hello!"`,
			},
			success: true,
			output:  "hello!\n",
		},
		{
			params: types.ExecScript{
				Type: "basic-env",
				ID:   1,
				Env: map[string]string{
					"name": "alice",
				},
				Script: `echo "Hello ${name}!"`,
			},
			success: true,
			output:  "Hello alice!\n",
		},
		{
			params: types.ExecScript{
				Type:   "python-shebang",
				ID:     1,
				Shell:  `/usr/bin/env python3`,
				Script: `print("Hello from python!")`,
			},
			success: true,
			output:  "Hello from python!\n",
		},
		{
			params: types.ExecScript{
				Type:   "nonexistent-shell",
				ID:     1,
				Shell:  `/this/does/not/exist`,
				Script: `nothing(here)`,
			},
			success: false,
		},
		{
			params: types.ExecScript{
				Type:   "passthrough-var-fail",
				ID:     1,
				Script: `set -u; echo "Hello ${passthrough_test_var}!"`,
			},
			success: false,
		},
		{
			params: types.ExecScript{
				Type: "passthrough-var-success",
				ID:   1,
				EnvPassthrough: []string{
					"passthrough_test_var",
				},
				Script: `set -u; echo "Hello ${passthrough_test_var}!"`,
			},
			success: true,
			output:  "Hello from parent!\n",
		},
		{
			params: types.ExecScript{
				Type:   "../malicious-type-name",
				ID:     1,
				Script: `echo "hello!"`,
			},
			success: false,
		},
		{
			params: types.ExecScript{
				Type: "invalid-env-key",
				ID:   1,
				Env: map[string]string{
					"invalid=name": "never",
				},
				Script: `echo "hello!"`,
			},
			success: false,
		},
		{
			params: types.ExecScript{
				Type: "missing-script",
				ID:   1,
			},
			success: false,
		},
		{
			params: types.ExecScript{
				Type:   "expected-target-match",
				ID:     1,
				Script: `echo "hello!"`,
				Expect: vc.NewTarget("v1.2.3"),
			},
			success: true,
			output:  "hello!\n",
		},
		{
			params: types.ExecScript{
				Type:   "expected-target-mismatch",
				ID:     1,
				Script: `echo "hello!"`,
				Expect: vc.NewTarget("v2.3.4"),
			},
			success: false,
		},
	}

	// set an env var to be used in env passthrough tests.
	os.Setenv("passthrough_test_var", "from parent")

	executor, err := NewExecutor(ExecutorConfig{
		Current: vc.NewTarget("v1.2.3"),
		Dir:     t.TempDir(),
	})
	require.NoError(t, err)

	for _, tt := range tts {

		result := executor.Exec(tt.params)

		require.Equal(t, tt.success, result.Success, "result=%+v", result)

		if tt.success {
			out, err := executor.LoadOutput(Ref{
				Type: tt.params.Type,
				ID:   tt.params.ID,
			})
			require.NoError(t, err, "result=%+v", result)

			require.Equal(t, tt.output, out, "result=%+v", result)
		}
	}
}

func TestExpireEntries(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()

	executor, err := NewExecutor(ExecutorConfig{
		Dir:   t.TempDir(),
		Clock: clock,
	})
	require.NoError(t, err)

	r1 := executor.Exec(types.ExecScript{
		Type:   "t",
		ID:     1,
		Script: `echo "one"`,
	})
	require.True(t, r1.Success)

	clock.Advance(time.Minute)

	r2 := executor.Exec(types.ExecScript{
		Type:   "t",
		ID:     2,
		Script: `echo "two"`,
	})
	require.True(t, r2.Success)

	// verify that both entries are present
	entries, err := executor.ListEntries()
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// perform an expiry that shouldn't remove either entry
	err = executor.ExpireEntries(time.Minute * 2)
	require.NoError(t, err)

	// verify that both entries are still present
	entries, err = executor.ListEntries()
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// advance s.t. one of the two entries are expired
	clock.Advance(time.Second * 90)
	err = executor.ExpireEntries(time.Minute * 2)
	require.NoError(t, err)

	// verify that the newer entry is still present
	entries, err = executor.ListEntries()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, entries[0], Ref{Type: "t", ID: 2})

	// advance s.t. final entry expires
	clock.Advance(time.Second * 60)
	err = executor.ExpireEntries(time.Minute * 2)
	require.NoError(t, err)

	// verify all entries removed
	entries, err = executor.ListEntries()
	require.NoError(t, err)
	require.Len(t, entries, 0)

}

func TestRefs(t *testing.T) {
	t.Parallel()

	tts := []struct {
		r Ref
		s string
	}{
		{
			r: Ref{
				Type: "basic-Ref",
				ID:   123,
			},
			s: "basic-Ref-123",
		},
		{
			r: Ref{
				Type: "big-num",
				ID:   math.MaxUint64,
			},
			s: "big-num-18446744073709551615",
		},
		{
			r: Ref{
				Type: "small-num",
				ID:   0,
			},
			s: "small-num-0",
		},
	}

	for _, tt := range tts {
		require.Equal(t, tt.s, tt.r.String())

		ref, ok := parseRef(tt.s)
		require.True(t, ok)
		require.Equal(t, tt.r, ref)
	}
}
