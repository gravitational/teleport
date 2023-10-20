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

package connection

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParseRedisAddress(f *testing.F) {
	f.Add("foo:1234")
	f.Add(URIScheme + "://foo")
	f.Add(URIScheme + "://foo:1234?mode=standalone")
	f.Add(URIScheme + "://foo:1234?mode=cluster")

	f.Fuzz(func(t *testing.T, addr string) {
		require.NotPanics(t, func() {
			ParseRedisAddress(addr)
		})
	})
}
