//go:build go1.18

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

package protocol

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzMongoRead(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("000\xa4000000000000"))

	f.Fuzz(func(t *testing.T, msgBytes []byte) {
		msg := bytes.NewReader(msgBytes)

		require.NotPanics(t, func() {
			_, _ = ReadMessage(msg)
		})
	})
}
