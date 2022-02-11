// Copyright 2022 Gravitational, Inc
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

package breaker

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestRoundTripper_RoundTrip(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	cb, err := New(Config{
		Clock:         clock,
		RecoveryLimit: 1,
		Interval:      time.Second,
		TrippedPeriod: time.Second,
		IsSuccessful: func(v interface{}, err error) bool {
			if err != nil {
				return false
			}

			if v == nil {
				return false
			}

			switch t := v.(type) {
			case *http.Response:
				return t.StatusCode < http.StatusInternalServerError
			}

			return true
		},
		Trip: ConsecutiveFailureTripper(1),
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("/success", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	srv := httptest.NewServer(mux)

	clt := srv.Client()
	clt.Transport = NewRoundTripper(cb, clt.Transport)

	ctx := context.Background()

	cases := []struct {
		desc              string
		url               string
		state             State
		advance           time.Duration
		errorAssertion    require.ErrorAssertionFunc
		responseAssertion require.ValueAssertionFunc
	}{
		{
			desc:           "success in standby",
			url:            "/success",
			state:          StateStandby,
			errorAssertion: require.NoError,
			responseAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, http.StatusOK, i.(*http.Response).StatusCode)
			},
		},
		{
			desc:  "error when tripped",
			url:   "/success",
			state: StateTripped,
			errorAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrStateTripped)
			},
			responseAssertion: require.Nil,
		},
		{
			desc:  "error when recovery limit exceeded",
			url:   "/success",
			state: StateRecovering,
			errorAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrRecoveryLimitExceeded)
			},
			responseAssertion: require.Nil,
		},
		{
			desc:           "allowed request when recovery progresses",
			url:            "/success",
			state:          StateRecovering,
			advance:        time.Minute,
			errorAssertion: require.NoError,
			responseAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, http.StatusOK, i.(*http.Response).StatusCode)
			},
		},
		{
			desc:           "failure in standby",
			url:            "/fail",
			state:          StateStandby,
			errorAssertion: require.NoError,
			responseAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, http.StatusBadGateway, i.(*http.Response).StatusCode)
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			cb.setState(tt.state, clock.Now())
			clock.Advance(tt.advance)

			r, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+tt.url, nil)
			require.NoError(t, err)
			resp, err := clt.Do(r)
			t.Cleanup(func() {
				require.NoError(t, resp.Body.Close())
			})
			tt.errorAssertion(t, err)
			tt.responseAssertion(t, resp)
		})
	}
}
