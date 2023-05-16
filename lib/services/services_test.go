/*
Copyright 2018 Gravitational, Inc.

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

package services

import (
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestOptions tests command options operations
func TestOptions(t *testing.T) {
	t.Parallel()

	// test empty scenario
	out := AddOptions(nil)
	require.Len(t, out, 0)

	// make sure original option list is not affected
	in := []MarshalOption{}
	out = AddOptions(in, WithResourceID(1))
	require.Len(t, out, 1)
	require.Len(t, in, 0)
	cfg, err := CollectOptions(out)
	require.NoError(t, err)
	require.Equal(t, cfg.ID, int64(1))

	// Add a couple of other parameters
	out = AddOptions(in, WithResourceID(2), WithVersion(types.V2))
	require.Len(t, out, 2)
	require.Len(t, in, 0)
	cfg, err = CollectOptions(out)
	require.NoError(t, err)
	require.Equal(t, cfg.ID, int64(2))
	require.Equal(t, cfg.Version, types.V2)
}

// TestCommandLabels tests command labels
func TestCommandLabels(t *testing.T) {
	t.Parallel()

	var l CommandLabels
	out := l.Clone()
	require.Len(t, out, 0)

	label := &types.CommandLabelV2{Command: []string{"ls", "-l"}, Period: types.Duration(time.Second)}
	l = CommandLabels{"a": label}
	out = l.Clone()

	require.Len(t, out, 1)
	require.Empty(t, cmp.Diff(out["a"], label))

	// make sure it's not a shallow copy
	label.Command[0] = "/bin/ls"
	require.NotEqual(t, label.Command[0], out["a"].GetCommand())
}

func TestLabelKeyValidation(t *testing.T) {
	t.Parallel()

	tts := []struct {
		label string
		ok    bool
	}{
		{label: "somelabel", ok: true},
		{label: "foo.bar", ok: true},
		{label: "this-that", ok: true},
		{label: "8675309", ok: true},
		{label: "", ok: false},
		{label: "spam:eggs", ok: true},
		{label: "cats dogs", ok: false},
		{label: "wut?", ok: false},
	}
	for _, tt := range tts {
		require.Equal(t, types.IsValidLabelKey(tt.label), tt.ok)
	}
}

func TestServerDeepCopy(t *testing.T) {
	t.Parallel()

	// setup
	now := time.Date(1984, time.April, 4, 0, 0, 0, 0, time.UTC)
	expires := now.Add(1 * time.Hour)
	srv := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "a",
			Namespace: apidefaults.Namespace,
			Labels:    map[string]string{"label": "value"},
			Expires:   &expires,
		},
		Spec: types.ServerSpecV2{
			Addr:     "127.0.0.1:0",
			Hostname: "hostname",
			CmdLabels: map[string]types.CommandLabelV2{
				"srv-cmd": {
					Period:  types.Duration(2 * time.Second),
					Command: []string{"srv-cmd", "--switch"},
				},
			},
			Rotation: types.Rotation{
				Started:     now,
				GracePeriod: types.Duration(1 * time.Minute),
				LastRotated: now.Add(-1 * time.Minute),
			},
			Apps: []*types.App{
				{
					Name:         "app",
					StaticLabels: map[string]string{"label": "value"},
					DynamicLabels: map[string]types.CommandLabelV2{
						"app-cmd": {
							Period:  types.Duration(1 * time.Second),
							Command: []string{"app-cmd", "--app-flag"},
						},
					},
					Rewrite: &types.Rewrite{
						Redirect: []string{"host1", "host2"},
					},
				},
			},
		},
	}

	// exercise
	srv2 := srv.DeepCopy()

	// verify
	require.Empty(t, cmp.Diff(srv, srv2))
	require.IsType(t, srv2, &types.ServerV2{})

	// Mutate the second value but expect the original to be unaffected
	srv2.(*types.ServerV2).Metadata.Labels["foo"] = "bar"
	srv2.(*types.ServerV2).Spec.CmdLabels = map[string]types.CommandLabelV2{
		"srv-cmd": {
			Period:  types.Duration(3 * time.Second),
			Command: []string{"cmd", "--flag=value"},
		},
	}
	expires2 := now.Add(10 * time.Minute)
	srv2.(*types.ServerV2).Metadata.Expires = &expires2

	// exercise
	srv3 := srv.DeepCopy()

	// verify
	require.Empty(t, cmp.Diff(srv, srv3))
	require.NotEmpty(t, cmp.Diff(srv.GetMetadata().Labels, srv2.GetMetadata().Labels))
	require.NotEmpty(t, cmp.Diff(srv2, srv3))
}
