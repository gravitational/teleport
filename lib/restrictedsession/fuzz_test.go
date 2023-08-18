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

package restrictedsession

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParseIPSpec(f *testing.F) {
	f.Add("127.0.0.111")
	f.Add("127.0.0.111/8")
	f.Add("192.168.0.0/16")
	f.Add("2001:0db8:85a3:0000:0000:8a2e:0370:7334")
	f.Add("2001:0db8:85a3:0000:0000:8a2e:0370:7334/64")
	f.Add("2001:db8::ff00:42:8329")
	f.Add("2001:db8::ff00:42:8329/48")

	f.Fuzz(func(t *testing.T, cidr string) {
		require.NotPanics(t, func() {
			ParseIPSpec(cidr)
		})
	})
}
