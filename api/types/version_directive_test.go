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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

type testTargets struct {
	Targets []WrappedInstallTarget `json:"targets,omitempty"`
}

const testTargetsJSON = `{"targets":[{"version":"v1.2.3"},{"security-patch":"yes","version":"v2.3.4"}]}`

const testTargetsYAML = `targets:
- version: v1.2.3
- security-patch: "yes"
  version: v2.3.4
`

// TestWRappedTargetMarshalling verifies that WrappedInstallTarget presents itself as a
// mapping in json and yaml marshal/unmarshal.
func TestWrappedTargetMarshalling(t *testing.T) {
	tts := []struct {
		json    string
		yaml    string
		ok      bool
		targets testTargets
	}{
		{
			json: testTargetsJSON,
			yaml: testTargetsYAML,
			ok:   true,
			targets: testTargets{
				Targets: []WrappedInstallTarget{
					{
						Target: map[string]string{
							"version": "v1.2.3",
						},
					},
					{
						Target: map[string]string{
							"version":        "v2.3.4",
							"security-patch": "yes",
						},
					},
				},
			},
		},
		{
			json: `{}`,
			yaml: "targets: []\n",
		},
	}

	for _, tt := range tts {
		data, err := yaml.Marshal(&tt.targets)
		require.NoError(t, err)
		require.Equal(t, []byte(tt.yaml), data)

		data, err = json.Marshal(&tt.targets)
		require.NoError(t, err)
		require.Equal(t, []byte(tt.json), data)

		var targets testTargets
		err = json.Unmarshal([]byte(tt.json), &targets)
		require.NoError(t, err)
		if len(tt.targets.Targets) == 0 {
			require.Len(t, targets.Targets, 0)
		} else {
			require.Equal(t, tt.targets, targets)
		}
		for _, w := range targets.Targets {
			require.Equal(t, tt.ok, w.Target.Ok())
		}

		targets = testTargets{}
		err = yaml.Unmarshal([]byte(tt.yaml), &targets)
		require.NoError(t, err)
		if len(tt.targets.Targets) == 0 {
			require.Len(t, targets.Targets, 0)
		} else {
			require.Equal(t, tt.targets, targets)
		}
		for _, w := range targets.Targets {
			require.Equal(t, tt.ok, w.Target.Ok())
		}
	}
}
