/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package stream_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/internalutils/stream"
	streamutils "github.com/gravitational/teleport/lib/utils/stream"
)

func TestMapStream(t *testing.T) {
	x := 0
	fn := func() (int, error) {
		if x == 5 {
			return 0, io.EOF
		}

		x = x + 1
		return x, nil
	}

	first := stream.Func(fn)
	mapped := streamutils.NewMapStreams(first, func(x int) int {
		return x * 2
	})

	values, err := stream.Collect(mapped)
	require.NoError(t, err)
	require.Equal(t, values, []int{2, 4, 6, 8, 10})
}
