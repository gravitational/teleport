// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebappsVersion(t *testing.T) {
	for _, test := range []struct {
		desc         string
		droneTag     string
		targetBranch string
		want         string
	}{
		{desc: "prefer tag", droneTag: "v9.2.0", want: "v9.2.0"},
		{desc: "maps branches", targetBranch: "branch/v9", want: "teleport-v9"},
		{desc: "fallback master", targetBranch: "foobar", want: "master"},
	} {
		t.Run(test.desc, func(t *testing.T) {
			require.Equal(t, test.want,
				webappsVersion(test.droneTag, test.targetBranch))
		})
	}
}
