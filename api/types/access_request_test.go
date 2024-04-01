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

func TestAccessRequestFilter(t *testing.T) {
	var reqs []AccessRequest
	req1, err := NewAccessRequest("0001", "bob", "test")
	require.NoError(t, err)
	reqs = append(reqs, req1)

	req2, err := NewAccessRequest("0002", "alice", "test")
	require.NoError(t, err)
	reqs = append(reqs, req2)

	req3, err := NewAccessRequest("0003", "alice", "test")
	req3.SetReviews([]AccessReview{{Author: "bob"}})
	require.NoError(t, err)
	reqs = append(reqs, req3)

	req4, err := NewAccessRequest("0004", "alice", "test")
	req4.SetReviews([]AccessReview{{Author: "bob"}})
	require.NoError(t, err)
	req4.SetState(RequestState_APPROVED)
	reqs = append(reqs, req4)

	req5, err := NewAccessRequest("0005", "jan", "test")
	require.NoError(t, err)
	req5.SetState(RequestState_DENIED)
	reqs = append(reqs, req5)

	req6, err := NewAccessRequest("0006", "jan", "test")
	require.NoError(t, err)
	reqs = append(reqs, req6)

	cases := []struct {
		name     string
		filter   AccessRequestFilter
		expected []string
	}{
		{
			name: "user wants their own requests",
			filter: AccessRequestFilter{
				Requester: "alice",
				Scope:     AccessRequestScope_MY_REQUESTS,
			},
			expected: []string{"0002", "0003", "0004"},
		},
		{
			name: "user wants requests they need to review",
			filter: AccessRequestFilter{
				Requester: "bob",
				Scope:     AccessRequestScope_NEEDS_REVIEW,
			},
			expected: []string{"0002", "0006"},
		},
		{
			name: "user wants all requests",
			filter: AccessRequestFilter{
				Requester: "bob",
			},
			expected: []string{"0001", "0002", "0003", "0004", "0005", "0006"},
		},
		{
			name: "user wants requests theyve reviewed",
			filter: AccessRequestFilter{
				Requester: "bob",
				Scope:     AccessRequestScope_REVIEWED,
			},
			expected: []string{"0003", "0004"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ids []string
			for _, req := range reqs {
				if tc.filter.Match(req) {
					ids = append(ids, req.GetName())
				}
			}
			require.Equal(t, tc.expected, ids)
		})
	}
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
