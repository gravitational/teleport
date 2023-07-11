/*
Copyright 2023 Gravitational, Inc.

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

package inventory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

func identityJitter() retryutils.Jitter {
	return func(d time.Duration) time.Duration {
		return d
	}
}

// TestNinthRampingJitter performs basic verification of the effect of ninthRampingJitter
// by verifying that its outputs fall into expected ranges.
func TestNinthRampingJitter(t *testing.T) {
	rj := ninthRampingJitter(100, identityJitter())

	var durations []time.Duration
	for i := 0; i < 110; i++ {
		durations = append(durations, rj(time.Minute))
	}

	for i, d := range durations {
		require.LessOrEqual(t, d, time.Minute)

		switch {
		case i < 20:
			require.InDelta(t, 15, d.Seconds(), 10, "i=%d,d=%s", i, d)
		case i < 40:
			require.InDelta(t, 25, d.Seconds(), 10, "i=%d,d=%s", i, d)
		case i < 60:
			require.InDelta(t, 35, d.Seconds(), 10, "i=%d,d=%s", i, d)
		case i < 80:
			require.InDelta(t, 45, d.Seconds(), 10, "i=%d,d=%s", i, d)
		default:
			require.InDelta(t, 55, d.Seconds(), 10, "i=%d,d=%s", i, d)
		}
	}
}
