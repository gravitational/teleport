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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	logmodels "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/logs"
)

// AG-mounted paths the detection commands hit. Kept as constants so the test
// fails loudly if the generated client's operation paths drift.
const (
	listAlertsPath = accessGraphAPIPath + "graph/alerts/v1"
)

// Fixed values shared across fixtures so handler assertions can pin the
// exact UUID / log entries they expect.
var (
	fixtureAlertID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	fixtureLogUIDs  = []string{"22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333"}
	fixtureStart    = time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	fixtureEnd      = time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	fixtureCreated  = time.Date(2026, 4, 2, 12, 5, 0, 0, time.UTC)
	fixtureUpdated  = time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC)
	fixtureFromArg  = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	fixtureToArg    = time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	fixtureLongDesc = strings.Repeat("very long detection write-up. ", 10) // >descriptionTextMaxLen runes
)

// alertFixture is a fully-populated SecurityAlert used as the default body for
// list/get fixtures. Individual tests mutate a copy to exercise edge cases
// (nil tags, missing AffectedEntity, empty LogEntries, etc.).
func alertFixture(t *testing.T) accessgraph.SecurityAlert {
	t.Helper()
	reporter := "detection-engine"
	desc := fixtureLongDesc
	entName := "alice@example.com"
	entType := "user"
	tags := []string{"prod", "iam"}
	mitigations := []string{"rotate-key", "notify-security"}
	logEntries := append([]string{}, fixtureLogUIDs...)
	updated := fixtureUpdated
	return accessgraph.SecurityAlert{
		Id:        fixtureAlertID,
		Title:     "Unusual privilege escalation",
		Type:      "privilege_escalation",
		Severity:  accessgraph.SecurityAlertSeverity("high"),
		Status:    accessgraph.AlertStatus("in_progress"),
		Source:    logmodels.EventSource("aws"),
		StartTime: fixtureStart,
		EndTime:   fixtureEnd,
		CreatedAt: fixtureCreated,
		UpdatedAt: &updated,
		AffectedEntity: &struct {
			Name *string `json:"name,omitempty"`
			Type *string `json:"type,omitempty"`
		}{Name: &entName, Type: &entType},
		Tags:            &tags,
		Description:     &desc,
		ReportedBy:      &reporter,
		LogEntries:      &logEntries,
		MitigationSteps: &mitigations,
		StatusChangeLogs: []accessgraph.AlertStatusChangeLog{{
			CreatedAt: fixtureCreated,
			Status:    accessgraph.AlertStatus("in_progress"),
			User:      "system",
		}},
	}
}

// listRequest captures what the list handler observed so tests can assert
// on the request shaping (query DSL, start/end time) produced by
// constructAlertsListQuery without re-parsing the wire format.
type listRequest struct {
	path  string
	query url.Values
}

// newListAlertsHandler serves a ListAlertsV1 response and records the inbound
// request into captured for assertion. Pass statusCode != 0 to drive the
// error-propagation paths through the same helper.
func newListAlertsHandler(t *testing.T, alerts []accessgraph.SecurityAlert, captured *listRequest, statusCode int, errBody string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if captured != nil {
			*captured = listRequest{path: r.URL.Path, query: r.URL.Query()}
		}
		require.Equal(t, listAlertsPath, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if statusCode != 0 && statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
			_, _ = w.Write([]byte(errBody))
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": alerts})
	})
}

// fetchAlertsPage is a single page returned by newPaginatedAlertsHandler.
type fetchAlertsPage struct {
	data       []accessgraph.SecurityAlert
	nextCursor *string
}

// newPaginatedAlertsHandler advances through pages on each request, asserting
// the inbound next_cursor matches the previous page's signal.
func newPaginatedAlertsHandler(t *testing.T, pages []fetchAlertsPage) http.Handler {
	t.Helper()
	var calls atomic.Int64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(calls.Add(1) - 1)
		require.Less(t, idx, len(pages), "more page requests than configured")
		if idx > 0 {
			prev := pages[idx-1].nextCursor
			require.NotNil(t, prev, "fetchAlerts continued past a nil cursor")
			require.Equal(t, *prev, r.URL.Query().Get("next_cursor"))
		}
		page := pages[idx]
		body := map[string]any{"data": page.data}
		if page.nextCursor != nil {
			body["next_cursor"] = *page.nextCursor
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(body)
	})
}

// newDetectionsCommand wires an AccessGraphCommand with a captured stdout
// buffer and the supplied defaults. Tests override fields on the returned
// command before calling DetectionsList directly — TryRun is intentionally
// bypassed because it owns credential loading, not behavior.
func newDetectionsCommand(t *testing.T, format string) (*AccessGraphCommand, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	c := &AccessGraphCommand{
		stdout: &buf,
		detections: detectionsArgs{
			format: format,
			from:   fixtureFromArg,
			to:     fixtureToArg,
		},
	}
	return c, &buf
}

// TestDetectionsList covers DetectionsList: request shaping into the AG list
// endpoint, output rendering across formats, and error propagation.
func TestDetectionsList(t *testing.T) {
	t.Parallel()

	t.Run("request shaping carries filters and window", func(t *testing.T) {
		t.Parallel()
		var got listRequest
		ag := newAccessGraphTestClient(t, newListAlertsHandler(t,
			[]accessgraph.SecurityAlert{alertFixture(t)}, &got, 0, ""))

		c, _ := newDetectionsCommand(t, teleport.JSON)
		c.detections.ls = detectionsListArgs{
			status:   []string{"in_progress", "triaged"},
			source:   []string{"aws"},
			typ:      []string{"privilege_escalation"},
			severity: []string{"high", "critical"},
		}
		require.NoError(t, c.DetectionsList(context.Background(), ag))

		// The DSL is assembled by ranging over a map, so the AND-joined
		// clauses come in non-deterministic order — assert membership.
		clauses := strings.Split(got.query.Get("query"), " AND ")
		require.ElementsMatch(t, []string{
			`status:("in_progress" OR "triaged")`,
			`source:"aws"`,
			`type:"privilege_escalation"`,
			`severity:("high" OR "critical")`,
		}, clauses)

		gotStart, err := time.Parse(time.RFC3339Nano, got.query.Get("start_time"))
		require.NoError(t, err)
		require.True(t, gotStart.Equal(fixtureFromArg), "got %v want %v", gotStart, fixtureFromArg)
		gotEnd, err := time.Parse(time.RFC3339Nano, got.query.Get("end_time"))
		require.NoError(t, err)
		require.True(t, gotEnd.Equal(fixtureToArg), "got %v want %v", gotEnd, fixtureToArg)
	})

	t.Run("json format emits the alert list verbatim", func(t *testing.T) {
		t.Parallel()
		ag := newAccessGraphTestClient(t, newListAlertsHandler(t,
			[]accessgraph.SecurityAlert{alertFixture(t)}, nil, 0, ""))

		c, buf := newDetectionsCommand(t, teleport.JSON)
		require.NoError(t, c.DetectionsList(context.Background(), ag))

		var out []accessgraph.SecurityAlert
		require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
		require.Len(t, out, 1)
		require.Equal(t, fixtureAlertID, out[0].Id)
		require.Equal(t, "Unusual privilege escalation", out[0].Title)
	})

	t.Run("yaml format emits the alert list", func(t *testing.T) {
		t.Parallel()
		ag := newAccessGraphTestClient(t, newListAlertsHandler(t,
			[]accessgraph.SecurityAlert{alertFixture(t)}, nil, 0, ""))

		c, buf := newDetectionsCommand(t, teleport.YAML)
		require.NoError(t, c.DetectionsList(context.Background(), ag))

		// utils.WriteYAML emits a slice of length 1 as the bare item (no list wrapper),
		// so we unmarshal into a single alert. Multi-doc YAML would need yaml.Decoder.
		var out accessgraph.SecurityAlert
		require.NoError(t, yaml.Unmarshal(buf.Bytes(), &out))
		require.Equal(t, fixtureAlertID, out.Id)
	})

	t.Run("text format renders the default column set", func(t *testing.T) {
		t.Parallel()
		ag := newAccessGraphTestClient(t, newListAlertsHandler(t,
			[]accessgraph.SecurityAlert{alertFixture(t)}, nil, 0, ""))

		c, buf := newDetectionsCommand(t, teleport.Text)
		require.NoError(t, c.DetectionsList(context.Background(), ag))

		out := buf.String()
		for _, h := range detectionRowHeaders(false) {
			require.Contains(t, out, h, "missing default column %q", h)
		}
		// Detailed-only columns must NOT appear in the default view.
		for _, h := range []string{"Affected Entity", "Tags", "Description", "Reported By"} {
			require.NotContains(t, out, h, "detailed column %q leaked into default view", h)
		}
		// ID and the Alert title both render in the default view (possibly clipped by the truncated-column helper).
		require.Contains(t, out, "11111111", "alert ID prefix should appear")
		require.Contains(t, out, "Unusual", "alert title prefix should appear under the Alert column")
	})

	t.Run("text format with --detailed adds detailed columns and avoids leaking full description", func(t *testing.T) {
		t.Parallel()
		ag := newAccessGraphTestClient(t, newListAlertsHandler(t,
			[]accessgraph.SecurityAlert{alertFixture(t)}, nil, 0, ""))

		c, buf := newDetectionsCommand(t, teleport.Text)
		c.detections.ls.detailed = true
		require.NoError(t, c.DetectionsList(context.Background(), ag))

		out := buf.String()
		for _, h := range detectionRowHeaders(true) {
			require.Contains(t, out, h, "missing detailed column %q", h)
		}
		// Long description must not appear in full — table renderer / pre-truncation clips it.
		require.NotContains(t, out, fixtureLongDesc, "untruncated description leaked into the table")
	})

	t.Run("text format on empty alert list renders only headers", func(t *testing.T) {
		t.Parallel()
		ag := newAccessGraphTestClient(t, newListAlertsHandler(t, []accessgraph.SecurityAlert{}, nil, 0, ""))

		c, buf := newDetectionsCommand(t, teleport.Text)
		require.NoError(t, c.DetectionsList(context.Background(), ag))
		out := buf.String()
		for _, h := range detectionRowHeaders(false) {
			require.Contains(t, out, h, "header %q should still render for empty result", h)
		}
		require.NotContains(t, out, fixtureAlertID.String(), "no alert rows expected for empty result")
	})

	t.Run("nil alert list renders as empty", func(t *testing.T) {
		t.Parallel()
		// Handler returns {"data": null} — displayDetections normalizes
		// this to an empty slice so JSON callers don't get a `null` body.
		ag := newAccessGraphTestClient(t, newListAlertsHandler(t, nil, nil, 0, ""))

		c, buf := newDetectionsCommand(t, teleport.JSON)
		require.NoError(t, c.DetectionsList(context.Background(), ag))
		require.Equal(t, "[]", strings.TrimSpace(buf.String()))
	})

	t.Run("HTTP 500 surfaces as APIResponseError", func(t *testing.T) {
		t.Parallel()
		ag := newAccessGraphTestClient(t, newListAlertsHandler(t, nil, nil,
			http.StatusInternalServerError, `{"message":"alerts backend exploded"}`))

		c, _ := newDetectionsCommand(t, teleport.JSON)
		err := c.DetectionsList(context.Background(), ag)
		var agErr *apiResponseError
		require.ErrorAs(t, err, &agErr)
		require.Equal(t, http.StatusInternalServerError, agErr.StatusCode)
		require.Equal(t, "alerts backend exploded", agErr.Message)
	})
}

// TestInitDetectionsFlags exercises the kingpin wiring (defaults, enum
// validation, --from/--to via timeValue) without going through TryRun or
// touching the network.
func TestInitDetectionsFlags(t *testing.T) {
	t.Parallel()

	// parse builds a fresh app + command and parses argv, returning the
	// populated detectionsArgs for assertion. Kept inline so each subtest
	// gets an isolated parser state.
	parse := func(t *testing.T, argv ...string) (detectionsArgs, error) {
		t.Helper()
		app := kingpin.New("tctl-test", "")
		c := &AccessGraphCommand{}
		c.initDetections(app)
		_, err := app.Parse(argv)
		return c.detections, err
	}

	t.Run("ls defaults", func(t *testing.T) {
		t.Parallel()
		before := time.Now()
		got, err := parse(t, "detections", "ls")
		after := time.Now()
		require.NoError(t, err)

		require.Equal(t, teleport.Text, got.format)
		require.Equal(t, []string{"in_progress", "triaged"}, got.ls.status)
		require.Empty(t, got.ls.severity, "no default severity filter — unset means all")
		require.False(t, got.ls.detailed)
		require.Equal(t, 100, got.ls.limit)

		// --from defaults to "30d" → ~30d before parse time. Allow the
		// parse window (before..after) to absorb any clock drift.
		require.WithinRange(t, got.from, before.Add(-30*24*time.Hour-time.Second), after.Add(-30*24*time.Hour+time.Second))
		require.WithinRange(t, got.to, before, after.Add(time.Second))
	})

	t.Run("ls rejects unknown status enum", func(t *testing.T) {
		t.Parallel()
		_, err := parse(t, "detections", "ls", "--status", "bogus")
		require.Error(t, err)
		require.Contains(t, err.Error(), "enum value must be one of")
		require.Contains(t, err.Error(), "triaged")
	})

	t.Run("ls rejects unknown severity enum", func(t *testing.T) {
		t.Parallel()
		_, err := parse(t, "detections", "ls", "--severity", "extreme")
		require.Error(t, err)
		require.Contains(t, err.Error(), "enum value must be one of")
		require.Contains(t, err.Error(), "critical")
	})

	t.Run("ls rejects unknown format enum", func(t *testing.T) {
		t.Parallel()
		_, err := parse(t, "detections", "ls", "--format", "xml")
		require.Error(t, err)
		require.Contains(t, err.Error(), "enum value must be one of")
	})

	t.Run("ls accepts --detailed, --format, and --limit", func(t *testing.T) {
		t.Parallel()
		got, err := parse(t, "detections", "ls", "--detailed", "--format", teleport.JSON, "--limit", "250")
		require.NoError(t, err)
		require.True(t, got.ls.detailed)
		require.Equal(t, teleport.JSON, got.format)
		require.Equal(t, 250, got.ls.limit)
	})
}

// TestFetchAlerts covers the pagination loop fetchAlerts wraps around
// ListAlertsV1: cursor walk and the limit-based trim.
func TestFetchAlerts(t *testing.T) {
	t.Parallel()

	t.Run("single page under limit returns all alerts", func(t *testing.T) {
		t.Parallel()
		ag := newAccessGraphTestClient(t, newPaginatedAlertsHandler(t, []fetchAlertsPage{
			{data: []accessgraph.SecurityAlert{alertFixture(t)}},
		}))
		got, err := fetchAlerts(context.Background(), ag, accessgraph.ListAlertsV1Params{}, 100)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, fixtureAlertID, got[0].Id)
	})

	t.Run("multi-page response walks the cursor", func(t *testing.T) {
		t.Parallel()
		cursor := "page2"
		ag := newAccessGraphTestClient(t, newPaginatedAlertsHandler(t, []fetchAlertsPage{
			{data: []accessgraph.SecurityAlert{alertFixture(t)}, nextCursor: &cursor},
			{data: []accessgraph.SecurityAlert{alertFixture(t)}},
		}))
		got, err := fetchAlerts(context.Background(), ag, accessgraph.ListAlertsV1Params{}, 100)
		require.NoError(t, err)
		require.Len(t, got, 2)
	})

	t.Run("limit caps and trims across pages", func(t *testing.T) {
		t.Parallel()
		cursor := "page2"
		// Two pages of 3 alerts each, limit=4 → fetch both pages, trim to 4.
		page1 := []accessgraph.SecurityAlert{alertFixture(t), alertFixture(t), alertFixture(t)}
		page2 := []accessgraph.SecurityAlert{alertFixture(t), alertFixture(t), alertFixture(t)}
		ag := newAccessGraphTestClient(t, newPaginatedAlertsHandler(t, []fetchAlertsPage{
			{data: page1, nextCursor: &cursor},
			{data: page2},
		}))
		got, err := fetchAlerts(context.Background(), ag, accessgraph.ListAlertsV1Params{}, 4)
		require.NoError(t, err)
		require.Len(t, got, 4)
	})

	t.Run("limit zero disables the cap", func(t *testing.T) {
		t.Parallel()
		cursor := "page2"
		ag := newAccessGraphTestClient(t, newPaginatedAlertsHandler(t, []fetchAlertsPage{
			{data: []accessgraph.SecurityAlert{alertFixture(t), alertFixture(t)}, nextCursor: &cursor},
			{data: []accessgraph.SecurityAlert{alertFixture(t)}},
		}))
		got, err := fetchAlerts(context.Background(), ag, accessgraph.ListAlertsV1Params{}, 0)
		require.NoError(t, err)
		require.Len(t, got, 3)
	})
}
