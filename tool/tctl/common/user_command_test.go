// Copyright 2021 Gravitational, Inc
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

package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrimDurationSuffix(t *testing.T) {
	t.Parallel()
	var testCases = []struct {
		comment string
		ts      time.Duration
		wantFmt string
	}{
		{
			comment: "trim minutes/seconds",
			ts:      1 * time.Hour,
			wantFmt: "1h",
		},
		{
			comment: "trim seconds",
			ts:      1 * time.Minute,
			wantFmt: "1m",
		},
		{
			comment: "does not trim non-zero suffix",
			ts:      90 * time.Second,
			wantFmt: "1m30s",
		},
		{
			comment: "does not trim zero in the middle",
			ts:      3630 * time.Second,
			wantFmt: "1h0m30s",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.comment, func(t *testing.T) {
			fmt := trimDurationZeroSuffix(tt.ts)
			require.Equal(t, fmt, tt.wantFmt)
		})
	}
}
