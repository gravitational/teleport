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

package types

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
)

func TestAssertAccessRequestImplementsResourceWithLabels(t *testing.T) {
	ar, err := NewAccessRequest("test", "test", "test")
	require.NoError(t, err)
	require.Implements(t, (*ResourceWithLabels)(nil), ar)
}

func TestValidateAssumeStartTime(t *testing.T) {
	creation := time.Now().UTC()
	const day = 24 * time.Hour

	expiry := creation.Add(12 * day)
	maxAssumeStartDuration := creation.Add(constants.MaxAssumeStartDuration)

	testCases := []struct {
		name      string
		startTime time.Time
		errCheck  require.ErrorAssertionFunc
	}{
		{
			name:      "start time too far in the future",
			startTime: creation.Add(constants.MaxAssumeStartDuration + day),
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, trace.BadParameter("assume start time is too far in the future, latest time allowed is %v",
					maxAssumeStartDuration.Format(time.RFC3339)))
			},
		},
		{
			name:      "expired start time",
			startTime: creation.Add(100 * day),
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("assume start time must be prior to access expiry time at %v",
					expiry.Format(time.RFC3339)))
			},
		},
		{
			name:      "before creation start time",
			startTime: creation.Add(-10 * day),
			errCheck: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("assume start time has to be after %v",
					creation.Format(time.RFC3339)))
			},
		},
		{
			name:      "valid start time",
			startTime: creation.Add(6 * day),
			errCheck:  require.NoError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAssumeStartTime(tc.startTime, expiry, creation)
			tc.errCheck(t, err)
		})
	}
}
