/**
 * Copyright 2022 Gravitational, Inc.
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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidLabelKey(t *testing.T) {
	for _, tc := range []struct {
		label string
		valid bool
	}{
		{
			label: "1x/Y*_-",
			valid: true,
		},
		{
			label: "x:y",
			valid: true,
		},
		{
			label: "x\\y",
			valid: false,
		},
	} {
		isValid := IsValidLabelKey(tc.label)
		require.Equal(t, tc.valid, isValid)
	}
}
