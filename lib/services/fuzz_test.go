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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func FuzzParseRefs(f *testing.F) {
	f.Fuzz(func(t *testing.T, refs string) {
		require.NotPanics(t, func() {
			ParseRefs(refs)
		})
	})
}

func FuzzParserEvalBoolPredicate(f *testing.F) {
	f.Fuzz(func(t *testing.T, expr string) {
		resource, err := types.NewServerWithLabels("test-name", types.KindNode, types.ServerSpecV2{
			Hostname: "test-hostname",
			Addr:     "test-addr",
			CmdLabels: map[string]types.CommandLabelV2{
				"version": {
					Result: "v8",
				},
			},
		}, map[string]string{
			"env": "prod",
			"os":  "mac",
		})
		require.NoError(t, err)

		parser, err := NewResourceParser(resource)
		require.NoError(t, err)

		require.NotPanics(t, func() {
			parser.EvalBoolPredicate(expr)
		})
	})
}
