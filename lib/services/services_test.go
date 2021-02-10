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

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type ServicesSuite struct {
}

func TestServices(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&ServicesSuite{})

// TestCommandLabels tests command labels
func (s *ServicesSuite) TestCommandLabels(c *check.C) {
	var l CommandLabels
	out := l.Clone()
	c.Assert(out, check.HasLen, 0)

	label := &CommandLabelV2{Command: []string{"ls", "-l"}, Period: Duration(time.Second)}
	l = CommandLabels{"a": label}
	out = l.Clone()

	c.Assert(out, check.HasLen, 1)
	fixtures.DeepCompare(c, out["a"], label)

	// make sure it's not a shallow copy
	label.Command[0] = "/bin/ls"
	c.Assert(label.Command[0], check.Not(check.Equals), out["a"].GetCommand())
}

func (s *ServicesSuite) TestLabelKeyValidation(c *check.C) {
	tts := []struct {
		label string
		ok    bool
	}{
		{label: "somelabel", ok: true},
		{label: "foo.bar", ok: true},
		{label: "this-that", ok: true},
		{label: "8675309", ok: true},
		{label: "", ok: false},
		{label: "spam:eggs", ok: false},
		{label: "cats dogs", ok: false},
		{label: "wut?", ok: false},
	}
	for _, tt := range tts {
		c.Assert(IsValidLabelKey(tt.label), check.Equals, tt.ok, check.Commentf("tt=%+v", tt))
	}
}

func TestServerDeepCopy(t *testing.T) {
	t.Parallel()
	// setup
	now := time.Date(1984, time.April, 4, 0, 0, 0, 0, time.UTC)
	expires := now.Add(1 * time.Hour)
	srv := &ServerV2{
		Kind:    KindNode,
		Version: V2,
		Metadata: Metadata{
			Name:      "a",
			Namespace: defaults.Namespace,
			Labels:    map[string]string{"label": "value"},
			Expires:   &expires,
		},
		Spec: ServerSpecV2{
			Addr:     "127.0.0.1:0",
			Hostname: "hostname",
			CmdLabels: map[string]CommandLabelV2{
				"srv-cmd": {
					Period:  Duration(2 * time.Second),
					Command: []string{"srv-cmd", "--switch"},
				},
			},
			Rotation: Rotation{
				Started:     now,
				GracePeriod: Duration(1 * time.Minute),
				LastRotated: now.Add(-1 * time.Minute),
			},
			Apps: []*App{
				{
					Name:         "app",
					StaticLabels: map[string]string{"label": "value"},
					DynamicLabels: map[string]CommandLabelV2{
						"app-cmd": {
							Period:  Duration(1 * time.Second),
							Command: []string{"app-cmd", "--app-flag"},
						},
					},
					Rewrite: &Rewrite{
						Redirect: []string{"host1", "host2"},
					},
				},
			},
			KubernetesClusters: []*KubernetesCluster{
				{
					Name:         "cluster",
					StaticLabels: map[string]string{"label": "value"},
					DynamicLabels: map[string]CommandLabelV2{
						"cmd": {
							Period:  Duration(1 * time.Second),
							Command: []string{"cmd", "--flag"},
						},
					},
				},
			},
		},
	}

	// exercise
	srv2 := srv.DeepCopy()

	// verify
	require.Empty(t, cmp.Diff(srv, srv2))
	require.IsType(t, srv2, &ServerV2{})

	// Mutate the second value but expect the original to be unaffected
	srv2.(*ServerV2).Metadata.Labels["foo"] = "bar"
	srv2.(*ServerV2).Spec.CmdLabels = map[string]CommandLabelV2{
		"srv-cmd": {
			Period:  Duration(3 * time.Second),
			Command: []string{"cmd", "--flag=value"},
		},
	}
	expires2 := now.Add(10 * time.Minute)
	srv2.(*ServerV2).Metadata.Expires = &expires2

	// exercise
	srv3 := srv.DeepCopy()

	// verify
	require.Empty(t, cmp.Diff(srv, srv3))
	require.NotEmpty(t, cmp.Diff(srv.GetMetadata().Labels, srv2.GetMetadata().Labels))
	require.NotEmpty(t, cmp.Diff(srv2, srv3))
}
