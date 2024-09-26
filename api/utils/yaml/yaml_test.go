// Copyright 2024 Gravitational, Inc.
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

package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testStruct struct {
	X int    `yaml:"x"`
	Y string `yaml:"y"`
}

const yamlWithCorrectFields = `x: 1
y: foo
`

const yamlWithExtraFields = `x: 1
y: foo
z: bar
`

func TestUnmarshalStrict(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		yamlIn    []byte
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:      "all yaml fields match",
			yamlIn:    []byte(yamlWithCorrectFields),
			assertErr: assert.NoError,
		},
		{
			name:      "extra yaml fields",
			yamlIn:    []byte(yamlWithExtraFields),
			assertErr: assert.Error,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out testStruct
			err := UnmarshalStrict(tc.yamlIn, &out)
			tc.assertErr(t, err)
			assert.Equal(t, testStruct{X: 1, Y: "foo"}, out)
		})
	}
}
