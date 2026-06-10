/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package aws

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestGetAssumeRoleRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		request       *http.Request
		found         bool
		wantDuration  time.Duration
		actionInQuery bool
		actionInBody  bool
	}{
		{
			name:         "post form",
			request:      newAssumeRolePostRequest(t, "3600"),
			found:        true,
			wantDuration: time.Hour,
			actionInBody: true,
		},
		{
			name:          "query",
			request:       newAssumeRoleQueryRequest(t, "1800"),
			found:         true,
			wantDuration:  30 * time.Minute,
			actionInQuery: true,
		},
		{
			name:         "default duration",
			request:      newAssumeRolePostRequest(t, ""),
			found:        true,
			wantDuration: assumeRoleDefaultDuration,
			actionInBody: true,
		},
		{
			name: "uses longest duration from query and body",
			request: func() *http.Request {
				req := newAssumeRolePostRequest(t, "1800")
				query := req.URL.Query()
				query.Set(assumeRoleQueryKeyAction, assumeRoleQueryActionAssumeRole)
				query.Set(assumeRoleQueryKeyDurationSeconds, "7200")
				req.URL.RawQuery = query.Encode()
				return req
			}(),
			found:         true,
			wantDuration:  2 * time.Hour,
			actionInQuery: true,
			actionInBody:  true,
		},
		{
			name:    "not assume role",
			request: httptest.NewRequest(http.MethodPost, "https://sts.amazonaws.com/", strings.NewReader(url.Values{"Action": {"GetCallerIdentity"}}.Encode())),
			found:   false,
		},
		{
			name: "malformed post form",
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "https://sts.amazonaws.com/", strings.NewReader("Action=AssumeRole&DurationSeconds=%"))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return req
			}(),
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assumeRoleReq, err := getAssumeRoleRequest(tt.request)
			require.NoError(t, err)
			require.Equal(t, tt.found, assumeRoleReq != nil)
			if assumeRoleReq == nil {
				return
			}
			require.Equal(t, tt.wantDuration, assumeRoleReq.getDuration())
			require.Equal(t, tt.actionInQuery, assumeRoleReq.actionInQuery)
			require.Equal(t, tt.actionInBody, assumeRoleReq.actionInPostForm)
		})
	}
}

func TestRewriteAssumeRoleRequest(t *testing.T) {
	t.Parallel()

	t.Run("post form", func(t *testing.T) {
		t.Parallel()

		req := newAssumeRolePostRequest(t, "43200")
		assumeRoleReq, err := getAssumeRoleRequest(req)
		require.NoError(t, err)
		require.NotNil(t, assumeRoleReq)

		require.NoError(t, rewriteAssumeRoleRequest(req, assumeRoleReq, 30*time.Minute))
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		values, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		require.Equal(t, "1800", values.Get(assumeRoleQueryKeyDurationSeconds))
		require.Equal(t, int64(len(string(body))), req.ContentLength)
	})

	t.Run("query", func(t *testing.T) {
		t.Parallel()

		req := newAssumeRoleQueryRequest(t, "43200")
		req.RequestURI = req.URL.RequestURI()
		assumeRoleReq, err := getAssumeRoleRequest(req)
		require.NoError(t, err)
		require.NotNil(t, assumeRoleReq)

		require.NoError(t, rewriteAssumeRoleRequest(req, assumeRoleReq, 30*time.Minute))
		require.Equal(t, "1800", req.URL.Query().Get(assumeRoleQueryKeyDurationSeconds))
		require.Equal(t, req.URL.RequestURI(), req.RequestURI)
	})

	t.Run("query and post form", func(t *testing.T) {
		t.Parallel()

		req := newAssumeRolePostRequest(t, "1800")
		query := req.URL.Query()
		query.Set(assumeRoleQueryKeyAction, assumeRoleQueryActionAssumeRole)
		query.Set(assumeRoleQueryKeyDurationSeconds, "7200")
		req.URL.RawQuery = query.Encode()
		req.RequestURI = req.URL.RequestURI()
		assumeRoleReq, err := getAssumeRoleRequest(req)
		require.NoError(t, err)
		require.NotNil(t, assumeRoleReq)

		require.NoError(t, rewriteAssumeRoleRequest(req, assumeRoleReq, 45*time.Minute))

		require.Equal(t, "2700", req.URL.Query().Get(assumeRoleQueryKeyDurationSeconds))
		require.Equal(t, req.URL.RequestURI(), req.RequestURI)

		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		values, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		require.Equal(t, "2700", values.Get(assumeRoleQueryKeyDurationSeconds))
		require.Equal(t, int64(len(string(body))), req.ContentLength)
	})
}

func TestUpdateAssumeRoleDuration(t *testing.T) {
	t.Parallel()

	t.Run("explicit duration", func(t *testing.T) {
		t.Parallel()

		clock := clockwork.NewFakeClockAt(time.Now())
		identity := &tlsca.Identity{Expires: clock.Now().Add(time.Hour)}
		req := newAssumeRolePostRequest(t, "43200")
		w := httptest.NewRecorder()

		require.NoError(t, updateAssumeRoleDuration(identity, w, req, clock))
		require.Contains(t, w.Header().Get(common.TeleportAPIInfoHeader), "requested DurationSeconds of AssumeRole is lowered")

		assumeRoleReq, err := getAssumeRoleRequest(req)
		require.NoError(t, err)
		require.NotNil(t, assumeRoleReq)
		require.Equal(t, time.Hour, assumeRoleReq.getDuration())
	})

	t.Run("default duration", func(t *testing.T) {
		t.Parallel()

		clock := clockwork.NewFakeClockAt(time.Now())
		identity := &tlsca.Identity{Expires: clock.Now().Add(30 * time.Minute)}
		req := newAssumeRolePostRequest(t, "")
		w := httptest.NewRecorder()

		require.NoError(t, updateAssumeRoleDuration(identity, w, req, clock))
		require.Contains(t, w.Header().Get(common.TeleportAPIInfoHeader), "requested DurationSeconds of AssumeRole is lowered")

		assumeRoleReq, err := getAssumeRoleRequest(req)
		require.NoError(t, err)
		require.NotNil(t, assumeRoleReq)
		require.Equal(t, 30*time.Minute, assumeRoleReq.getDuration())
	})
}

func TestDenyLongAssumeRoleDuration(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Now())
	identity := &tlsca.Identity{Expires: clock.Now().Add(time.Hour)}

	require.NoError(t, denyLongAssumeRoleDuration(identity, newAssumeRolePostRequest(t, "1800"), clock))

	err := denyLongAssumeRoleDuration(identity, newAssumeRolePostRequest(t, "7200"), clock)
	require.True(t, trace.IsAccessDenied(err), "got %v", err)

	req := newAssumeRolePostRequest(t, "1800")
	query := req.URL.Query()
	query.Set(assumeRoleQueryKeyDurationSeconds, "7200")
	req.URL.RawQuery = query.Encode()
	err = denyLongAssumeRoleDuration(identity, req, clock)
	require.True(t, trace.IsAccessDenied(err), "got %v", err)

	err = denyLongAssumeRoleDuration(&tlsca.Identity{Expires: clock.Now().Add(10 * time.Minute)}, newAssumeRolePostRequest(t, "900"), clock)
	require.True(t, trace.IsAccessDenied(err), "got %v", err)
}

func newAssumeRolePostRequest(t *testing.T, durationSeconds string) *http.Request {
	t.Helper()

	form := url.Values{
		assumeRoleQueryKeyAction: {assumeRoleQueryActionAssumeRole},
		"RoleArn":                {"arn:aws:iam::123456789012:role/test-role"},
		"RoleSessionName":        {"test-session"},
	}
	if durationSeconds != "" {
		form.Set(assumeRoleQueryKeyDurationSeconds, durationSeconds)
	}
	req := httptest.NewRequest(http.MethodPost, "https://sts.amazonaws.com/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func newAssumeRoleQueryRequest(t *testing.T, durationSeconds string) *http.Request {
	t.Helper()

	query := url.Values{
		assumeRoleQueryKeyAction: {assumeRoleQueryActionAssumeRole},
		"RoleArn":                {"arn:aws:iam::123456789012:role/test-role"},
		"RoleSessionName":        {"test-session"},
	}
	if durationSeconds != "" {
		query.Set(assumeRoleQueryKeyDurationSeconds, durationSeconds)
	}
	return httptest.NewRequest(http.MethodGet, "https://sts.amazonaws.com/?"+query.Encode(), http.NoBody)
}
