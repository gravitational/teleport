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

package parse

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzNewExpression(f *testing.F) {
	f.Add("")
	f.Add("foo")
	f.Add("{{external.foo}}")
	f.Add(`{{regexp.replace(internal.foo, "foo-(.*)-(.*)", "$1.$2")}}`)

	f.Fuzz(func(t *testing.T, variable string) {
		require.NotPanics(t, func() {
			NewTraitsTemplateExpression(variable)
		})
	})
}

func FuzzNewMatcher(f *testing.F) {
	f.Add("")
	f.Add("foo")
	f.Add("*")
	f.Add("^foo$")
	f.Add(`{{regexp.match("foo.*")}}`)

	f.Fuzz(func(t *testing.T, value string) {
		require.NotPanics(t, func() {
			NewMatcher(value)
		})
	})
}
