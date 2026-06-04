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

package accessgraph

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
)

func TestParseRelativeDuration(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    time.Duration
		wantErr bool
	}{
		{name: "hours", in: "24h", want: 24 * time.Hour},
		{name: "minutes", in: "30m", want: 30 * time.Minute},
		{name: "compound", in: "1h30m", want: 90 * time.Minute},
		{name: "days", in: "7d", want: 7 * 24 * time.Hour},
		{name: "single day", in: "1d", want: 24 * time.Hour},
		{name: "fractional days", in: "0.5d", want: 12 * time.Hour},
		{name: "zero seconds", in: "0s", want: 0},
		{name: "empty string", in: "", wantErr: true},
		{name: "garbage", in: "abc", wantErr: true},
		{name: "lone d", in: "d", wantErr: true},
		{name: "no unit", in: "5", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRelativeDuration(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseTimeFilterValue(t *testing.T) {
	// Pinned "now" so the relative-duration cases are deterministic.
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	t.Run("empty string returns zero time", func(t *testing.T) {
		got, err := parseTimeFilterValue("", now)
		require.NoError(t, err)
		require.True(t, got.IsZero())
	})

	t.Run("RFC3339 timestamp parses as-is", func(t *testing.T) {
		want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		got, err := parseTimeFilterValue("2026-01-02T03:04:05Z", now)
		require.NoError(t, err)
		require.True(t, got.Equal(want), "got %v want %v", got, want)
	})

	t.Run("date-only input parses as local midnight", func(t *testing.T) {
		want := time.Date(2026, 5, 6, 0, 0, 0, 0, time.Local)
		got, err := parseTimeFilterValue("2026-05-06", now)
		require.NoError(t, err)
		require.True(t, got.Equal(want), "got %v want %v", got, want)
	})

	t.Run("\"now\" sentinel returns parse-time now", func(t *testing.T) {
		got, err := parseTimeFilterValue("now", now)
		require.NoError(t, err)
		require.Equal(t, now, got)
	})

	t.Run("relative duration is subtracted from now", func(t *testing.T) {
		got, err := parseTimeFilterValue("24h", now)
		require.NoError(t, err)
		require.Equal(t, now.Add(-24*time.Hour), got)
	})

	t.Run("negative duration is future-relative", func(t *testing.T) {
		// Documented behavior: a negative offset flips the subtraction
		// and produces a future timestamp. Used by callers that want to
		// query forward from now.
		got, err := parseTimeFilterValue("-2h", now)
		require.NoError(t, err)
		require.Equal(t, now.Add(2*time.Hour), got)
	})

	t.Run("relative day suffix", func(t *testing.T) {
		got, err := parseTimeFilterValue("7d", now)
		require.NoError(t, err)
		require.Equal(t, now.Add(-7*24*time.Hour), got)
	})

	t.Run("invalid input returns BadParameter", func(t *testing.T) {
		_, err := parseTimeFilterValue("not a time", now)
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
		require.Contains(t, err.Error(), "RFC3339")
	})
}

func TestTimeValue(t *testing.T) {
	t.Run("Set parses RFC3339 and String round-trips", func(t *testing.T) {
		var target time.Time
		v := timeValue{target: &target}
		require.NoError(t, v.Set("2026-01-02T03:04:05Z"))
		require.Equal(t, "2026-01-02T03:04:05Z", v.String())
	})

	t.Run("zero target prints empty", func(t *testing.T) {
		var target time.Time
		require.Empty(t, timeValue{target: &target}.String())
	})

	t.Run("nil target prints empty", func(t *testing.T) {
		require.Empty(t, timeValue{}.String())
	})

	t.Run("Set propagates parse errors", func(t *testing.T) {
		var target time.Time
		require.Error(t, timeValue{target: &target}.Set("garbage"))
	})
}

func TestValidateTimeWindow(t *testing.T) {
	earlier := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

	t.Run("from before to is valid", func(t *testing.T) {
		require.NoError(t, validateTimeWindow(earlier, later))
	})

	t.Run("from equal to is rejected", func(t *testing.T) {
		err := validateTimeWindow(earlier, earlier)
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
	})

	t.Run("from after to is rejected", func(t *testing.T) {
		err := validateTimeWindow(later, earlier)
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
	})

	t.Run("zero from is skipped", func(t *testing.T) {
		require.NoError(t, validateTimeWindow(time.Time{}, later))
	})

	t.Run("zero to is skipped", func(t *testing.T) {
		require.NoError(t, validateTimeWindow(earlier, time.Time{}))
	})
}

func TestStrPtrToStr(t *testing.T) {
	require.Empty(t, strPtrToStr(nil))
	empty := ""
	require.Empty(t, strPtrToStr(&empty))
	val := "hello"
	require.Equal(t, "hello", strPtrToStr(&val))
}

func TestDslClause(t *testing.T) {
	require.Empty(t, dslClause("user", nil))
	require.Empty(t, dslClause("user", []string{}))
	require.Equal(t, `user:"alice"`, dslClause("user", []string{"alice"}))
	require.Equal(t,
		`user:("alice" OR "bob")`,
		dslClause("user", []string{"alice", "bob"}),
	)
	// Embedded quote → \" (important: would otherwise break the DSL).
	require.Equal(t, `f:"a\"b"`, dslClause("f", []string{`a"b`}))
	// Backslash → \\.
	require.Equal(t, `f:"a\\b"`, dslClause("f", []string{`a\b`}))
	// Newline → \n escape sequence.
	require.Equal(t, `f:"a\nb"`, dslClause("f", []string{"a\nb"}))
	// Valid non-ASCII passes through unchanged (printable runes).
	require.Equal(t, `f:"café"`, dslClause("f", []string{"café"}))
}

type fetchAllLogsPage struct {
	data       []map[string]any
	nextCursor *string
}

// fetchAllLogsHandler returns an http.Handler that serves a fixed sequence
// of paginated responses. It records each request so tests can assert on
// the cursor flow.
func fetchAllLogsHandler(t *testing.T, pages []fetchAllLogsPage) (http.Handler, *atomic.Int64, *[]string) {
	t.Helper()
	var calls atomic.Int64
	cursors := []string{}
	cursorsRef := &cursors

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(calls.Add(1) - 1)
		// Record the iterator the client sent on this call.
		*cursorsRef = append(*cursorsRef, r.URL.Query().Get("iterator"))

		require.Less(t, idx, len(pages), "client requested more pages than configured")
		page := pages[idx]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":        page.data,
			"next_cursor": page.nextCursor,
		})
	})
	return handler, &calls, cursorsRef
}

func newAccessGraphTestClient(t *testing.T, h http.Handler) *accessgraph.ClientWithResponses {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)

	httpClient := srv.Client()
	if tr, ok := httpClient.Transport.(*http.Transport); ok {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	c, err := accessgraph.NewClientWithResponses(
		srv.URL+accessGraphAPIPath,
		accessgraph.WithHTTPClient(httpClient),
	)
	require.NoError(t, err)
	return c
}

func TestFetchAllLogs(t *testing.T) {
	t.Run("single page, no cursor → no pagination", func(t *testing.T) {
		h, calls, cursors := fetchAllLogsHandler(t, []fetchAllLogsPage{
			{data: []map[string]any{{"id": "a"}, {"id": "b"}}, nextCursor: nil},
		})
		c := newAccessGraphTestClient(t, h)

		got, truncated, err := fetchAllLogs(context.Background(), c, accessgraph.ExecuteLogsQueryV1Params{}, 10)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Len(t, got, 2)
		require.EqualValues(t, 1, calls.Load())
		require.Equal(t, []string{""}, *cursors, "first call must omit the iterator")
	})

	t.Run("multiple pages walk the cursor", func(t *testing.T) {
		c1 := "cursor-1"
		c2 := "cursor-2"
		h, calls, cursors := fetchAllLogsHandler(t, []fetchAllLogsPage{
			{data: []map[string]any{{"id": "a"}}, nextCursor: &c1},
			{data: []map[string]any{{"id": "b"}}, nextCursor: &c2},
			{data: []map[string]any{{"id": "c"}}, nextCursor: nil},
		})
		c := newAccessGraphTestClient(t, h)

		got, truncated, err := fetchAllLogs(context.Background(), c, accessgraph.ExecuteLogsQueryV1Params{}, 10)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Len(t, got, 3)
		require.EqualValues(t, 3, calls.Load())
		// Each subsequent call sends the previous response's
		// next_cursor as `iterator` — that's the contract.
		require.Equal(t, []string{"", "cursor-1", "cursor-2"}, *cursors)
	})

	t.Run("HTTP error short-circuits", func(t *testing.T) {
		var calls atomic.Int64
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"message":"server is sad"}`))
		})
		c := newAccessGraphTestClient(t, h)

		_, _, err := fetchAllLogs(context.Background(), c, accessgraph.ExecuteLogsQueryV1Params{}, 10)
		require.Error(t, err)
		var agErr *apiResponseError
		require.ErrorAs(t, err, &agErr)
		require.Equal(t, 500, agErr.StatusCode)
		require.Equal(t, "server is sad", agErr.Message)
		require.EqualValues(t, 1, calls.Load(), "must not retry past the first failure")
	})

	t.Run("maxResults limit returns partial results with truncated=true", func(t *testing.T) {
		// Advancing cursors per page so the non-advancing guard doesn't fire;
		// the cap must be the thing that stops the loop.
		cursors := []string{"c1", "c2", "c3", "c4", "c5"}
		var calls atomic.Int64
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := calls.Add(1)
			next := cursors[n-1]
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":        []map[string]any{{"id": "x"}},
				"next_cursor": &next,
			})
		})
		c := newAccessGraphTestClient(t, h)

		got, truncated, err := fetchAllLogs(context.Background(), c, accessgraph.ExecuteLogsQueryV1Params{}, 5)
		require.NoError(t, err)
		require.True(t, truncated, "more pages remained when maxResults was hit")
		require.Len(t, got, 5, "should return up to maxResults events")
		require.EqualValues(t, 5, calls.Load())
	})

	t.Run("page overshoot trims and flags truncated", func(t *testing.T) {
		h, calls, _ := fetchAllLogsHandler(t, []fetchAllLogsPage{
			{data: []map[string]any{{"id": "a"}, {"id": "b"}, {"id": "c"}, {"id": "d"}, {"id": "e"}}, nextCursor: nil},
		})
		c := newAccessGraphTestClient(t, h)

		got, truncated, err := fetchAllLogs(context.Background(), c, accessgraph.ExecuteLogsQueryV1Params{}, 3)
		require.NoError(t, err)
		require.True(t, truncated, "single-page overshoot must flag truncated")
		require.Len(t, got, 3)
		require.EqualValues(t, 1, calls.Load())
	})

	t.Run("non-advancing cursor breaks the unbounded loop", func(t *testing.T) {
		stuck := "stuck"
		var calls atomic.Int64
		// Always returns the same non-nil cursor, which without the guard
		// would loop forever when maxResults is unbounded.
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":        []map[string]any{{"id": "x"}},
				"next_cursor": &stuck,
			})
		})
		c := newAccessGraphTestClient(t, h)

		got, truncated, err := fetchAllLogs(context.Background(), c, accessgraph.ExecuteLogsQueryV1Params{}, 0)
		require.NoError(t, err)
		require.True(t, truncated, "non-advancing cursor must signal truncation")
		require.Len(t, got, 1, "guard fires before appending the duplicate page")
		require.EqualValues(t, 2, calls.Load(), "loop must stop on the first non-advancing cursor")
	})

	t.Run("maxResults zero means unbounded", func(t *testing.T) {
		c1 := "cursor-1"
		h, calls, _ := fetchAllLogsHandler(t, []fetchAllLogsPage{
			{data: []map[string]any{{"id": "a"}, {"id": "b"}}, nextCursor: &c1},
			{data: []map[string]any{{"id": "c"}}, nextCursor: nil},
		})
		c := newAccessGraphTestClient(t, h)

		got, truncated, err := fetchAllLogs(context.Background(), c, accessgraph.ExecuteLogsQueryV1Params{}, 0)
		require.NoError(t, err)
		require.False(t, truncated, "unbounded fetch must never report truncation")
		require.Len(t, got, 3)
		require.EqualValues(t, 2, calls.Load())
	})

	t.Run("maxResults equal to total — exhaustion, not truncation", func(t *testing.T) {
		c1 := "cursor-1"
		h, _, _ := fetchAllLogsHandler(t, []fetchAllLogsPage{
			{data: []map[string]any{{"id": "a"}, {"id": "b"}}, nextCursor: &c1},
			{data: []map[string]any{{"id": "c"}}, nextCursor: nil},
		})
		c := newAccessGraphTestClient(t, h)

		got, truncated, err := fetchAllLogs(context.Background(), c, accessgraph.ExecuteLogsQueryV1Params{}, 3)
		require.NoError(t, err)
		require.False(t, truncated, "natural exhaustion at the limit must not flag truncated")
		require.Len(t, got, 3)
	})

	t.Run("query and time params reach the server", func(t *testing.T) {
		var got url.Values
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"data":[]}`))
		})
		c := newAccessGraphTestClient(t, h)

		query := "user:alice"
		start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
		_, _, err := fetchAllLogs(context.Background(), c, accessgraph.ExecuteLogsQueryV1Params{
			Query:     &query,
			StartTime: &start,
			EndTime:   &end,
		}, 10)
		require.NoError(t, err)
		require.Equal(t, "user:alice", got.Get("query"))
		require.NotEmpty(t, got.Get("start_time"))
		require.NotEmpty(t, got.Get("end_time"))
	})
}

// TestWriteOutput covers the format dispatch for the writeOutput helper used
// by every AG consumer command.
func TestWriteOutput(t *testing.T) {
	payload := map[string]any{"id": "abc", "title": "hello"}
	textRender := func(out string) func(io.Writer) error {
		return func(w io.Writer) error {
			_, err := io.WriteString(w, out)
			return err
		}
	}

	t.Run("text invokes renderText and skips marshaling", func(t *testing.T) {
		var buf strings.Builder
		err := writeOutput(&buf, payload, teleport.Text, textRender("rendered text"))
		require.NoError(t, err)
		require.Equal(t, "rendered text", buf.String())
	})

	t.Run("json marshals payload and ignores renderText", func(t *testing.T) {
		var buf strings.Builder
		err := writeOutput(&buf, payload, teleport.JSON, func(io.Writer) error {
			t.Fatal("renderText must not be called for json")
			return nil
		})
		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(buf.String()), &got))
		require.Equal(t, payload, got)
	})

	t.Run("yaml marshals payload and ignores renderText", func(t *testing.T) {
		var buf strings.Builder
		err := writeOutput(&buf, payload, teleport.YAML, func(io.Writer) error {
			t.Fatal("renderText must not be called for yaml")
			return nil
		})
		require.NoError(t, err)
		require.Contains(t, buf.String(), "id: abc")
		require.Contains(t, buf.String(), "title: hello")
	})

	t.Run("unknown format returns BadParameter", func(t *testing.T) {
		var buf strings.Builder
		err := writeOutput(&buf, payload, "xml", textRender("unused"))
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
		require.Empty(t, buf.String())
	})

	t.Run("text propagates renderText error", func(t *testing.T) {
		want := trace.BadParameter("render failed")
		err := writeOutput(io.Discard, payload, teleport.Text, func(io.Writer) error {
			return want
		})
		require.ErrorIs(t, err, want)
	})
}
