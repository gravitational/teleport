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
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func TestProtoEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		inputA proto.Message
		inputB proto.Message
		assert require.BoolAssertionFunc
	}{
		{
			name: "true",
			inputA: &AWS{
				Region: "us-west-1",
				RedshiftServerless: RedshiftServerless{
					WorkgroupID: "id",
				},
			},
			inputB: &AWS{
				Region: "us-west-1",
				RedshiftServerless: RedshiftServerless{
					WorkgroupID: "id",
				},
			},
			assert: require.True,
		},
		{
			name: "true ignoring XXX_unrecognized",
			inputA: &AWS{
				Region:           "us-west-1",
				XXX_unrecognized: []byte{66, 0},
			},
			inputB: &AWS{
				Region: "us-west-1",
			},
			assert: require.True,
		},
		{
			name: "true ignoring nested XXX_unrecognized",
			inputA: &AWS{
				Region: "us-west-1",
				MemoryDB: MemoryDB{
					XXX_unrecognized: []byte{99, 0},
				},
			},
			inputB: &AWS{
				Region: "us-west-1",
			},
			assert: require.True,
		},
		{
			name: "false differrent values",
			inputA: &AWS{
				Region: "us-west-1",
				RedshiftServerless: RedshiftServerless{
					WorkgroupID: "id",
				},
			},
			inputB: &AWS{
				Region: "us-west-1",
				RedshiftServerless: RedshiftServerless{
					WorkgroupID: "differrent-id",
				},
			},
			assert: require.False,
		},
		{
			name: "false different types",
			inputA: &AWS{
				Region: "us-west-1",
			},
			inputB: &KubeAWS{
				Region: "us-west-1",
			},
			assert: require.False,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.assert(t, protoEqual(test.inputA, test.inputB))
		})
	}
}
