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

package changes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasCodeChanges(t *testing.T) {
	tests := []struct {
		name string
		in   Changes
		out  bool
	}{
		{
			name: "Only docs changes",
			in:   Changes{Docs: true},
			out:  false,
		},
		{
			name: "Only Go changes",
			in:   Changes{Code: true},
			out:  true,
		},
		{
			name: "Helm and docs changes",
			in:   Changes{Docs: true, Helm: true},
			out:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.out, test.in.HasCodeChanges())
		})
	}
}
