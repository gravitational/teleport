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
	"github.com/gravitational/trace"
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
	getAlertPath   = accessGraphAPIPath + "graph/alerts/v1/" // + <uuid>
	logsQueryPath  = accessGraphAPIPath + "graph/logs/v1"
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

// eventFixture builds a minimal event with a deterministic timestamp.
func eventFixture(uid string, offset time.Duration) logmodels.AccessgraphStorageV1alphaEvent {
	return logmodels.AccessgraphStorageV1alphaEvent{
		Uuid:        uid,
		Time:        fixtureStart.Add(offset),
		Action:      "LOGIN",
		EventType:   "authentication",
		EventSource: logmodels.EventSource("aws"),
		Status:      "success",
		Identity:    logmodels.AccessgraphStorageV1alphaIdentity{Name: "alice", Id: "alice@example.com"},
		Target:      logmodels.AccessgraphStorageV1alphaTarget{Resource: "prod-db"},
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

// getAlertRequests records the wire-level traffic DetectionsGet drove.
type getAlertRequests struct {
	alertPath    string
	logCalls     atomic.Int64
	logIterators []string
	logQuery     string
	logStart     string
	logEnd       string
}

// newGetAlertHandler serves the alert and (paginated) logs routes. Pass
// nil/empty logPages to assert the logs route is never hit.
func newGetAlertHandler(t *testing.T, alert accessgraph.SecurityAlert, logPages []fetchAllLogsPage, captured *getAlertRequests, alertStatus int, alertErrBody string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == getAlertPath+alert.Id.String():
			if captured != nil {
				captured.alertPath = r.URL.Path
			}
			if alertStatus != 0 && alertStatus != http.StatusOK {
				w.WriteHeader(alertStatus)
				_, _ = w.Write([]byte(alertErrBody))
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": alert})
		case r.URL.Path == logsQueryPath:
			idx := int(captured.logCalls.Add(1) - 1)
			captured.logIterators = append(captured.logIterators, r.URL.Query().Get("iterator"))
			// Query + window are constant across pages so first-call values are sufficient.
			if idx == 0 {
				captured.logQuery = r.URL.Query().Get("query")
				captured.logStart = r.URL.Query().Get("start_time")
				captured.logEnd = r.URL.Query().Get("end_time")
			}
			require.Less(t, idx, len(logPages), "client requested more log pages than configured")
			page := logPages[idx]
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":        page.data,
				"next_cursor": page.nextCursor,
			})
		default:
			t.Errorf("unexpected request to %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
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

	t.Run("HTTP 500 surfaces as apiResponseError", func(t *testing.T) {
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

func TestDetectionsGet(t *testing.T) {
	t.Parallel()

	t.Run("invalid uuid returns BadParameter without hitting the server", func(t *testing.T) {
		t.Parallel()
		// Handler fails on any request — the uuid guard must trip first.
		var called atomic.Int64
		ag := newAccessGraphTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called.Add(1)
			t.Errorf("server reached despite invalid uuid: %s", r.URL.Path)
		}))

		c, _ := newDetectionsCommand(t, teleport.JSON)
		c.detections.get.id = "not-a-uuid"
		err := c.DetectionsGet(context.Background(), ag)
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
		require.EqualValues(t, 0, called.Load())
	})

	t.Run("alert with log entries walks the logs cursor across pages", func(t *testing.T) {
		t.Parallel()
		alert := alertFixture(t)
		cursor := "page-2"
		page1 := []logmodels.AccessgraphStorageV1alphaEvent{eventFixture(fixtureLogUIDs[0], time.Minute)}
		page2 := []logmodels.AccessgraphStorageV1alphaEvent{eventFixture(fixtureLogUIDs[1], 2*time.Minute)}
		var got getAlertRequests
		ag := newAccessGraphTestClient(t, newGetAlertHandler(t, alert, []fetchAllLogsPage{
			{data: eventsAsMaps(t, page1), nextCursor: &cursor},
			{data: eventsAsMaps(t, page2)},
		}, &got, 0, ""))

		c, buf := newDetectionsCommand(t, teleport.JSON)
		c.detections.get.id = alert.Id.String()
		require.NoError(t, c.DetectionsGet(context.Background(), ag))

		require.Equal(t, getAlertPath+alert.Id.String(), got.alertPath)
		require.EqualValues(t, 2, got.logCalls.Load(), "should walk both pages")
		require.Equal(t, []string{"", "page-2"}, got.logIterators, "second call must send the prior next_cursor")
		require.Equal(t, `uid:("22222222-2222-2222-2222-222222222222" OR "33333333-3333-3333-3333-333333333333")`, got.logQuery)
		require.Equal(t, fixtureStart.Format(time.RFC3339), got.logStart, "fetchAlertEvents must send the alert's StartTime")
		require.Equal(t, fixtureEnd.Format(time.RFC3339), got.logEnd, "fetchAlertEvents must send the alert's EndTime")

		var out detectionGetOutput
		require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
		require.Len(t, out.Events, 2, "events from both pages should appear in the payload")
	})

	t.Run("alert without log entries skips the logs fetch", func(t *testing.T) {
		t.Parallel()
		alert := alertFixture(t)
		empty := []string{}
		alert.LogEntries = &empty

		var got getAlertRequests
		ag := newAccessGraphTestClient(t, newGetAlertHandler(t, alert, nil, &got, 0, ""))

		c, _ := newDetectionsCommand(t, teleport.JSON)
		c.detections.get.id = alert.Id.String()
		require.NoError(t, c.DetectionsGet(context.Background(), ag))
		require.EqualValues(t, 0, got.logCalls.Load(), "logs endpoint must not be called when LogEntries is empty")
	})

	t.Run("json output contains alert and events", func(t *testing.T) {
		t.Parallel()
		alert := alertFixture(t)
		events := []logmodels.AccessgraphStorageV1alphaEvent{
			eventFixture(fixtureLogUIDs[0], time.Minute),
			eventFixture(fixtureLogUIDs[1], 2*time.Minute),
		}
		ag := newAccessGraphTestClient(t, newGetAlertHandler(t, alert, []fetchAllLogsPage{{
			data: eventsAsMaps(t, events),
		}}, &getAlertRequests{}, 0, ""))

		c, buf := newDetectionsCommand(t, teleport.JSON)
		c.detections.get.id = alert.Id.String()
		require.NoError(t, c.DetectionsGet(context.Background(), ag))

		var out detectionGetOutput
		require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
		require.Equal(t, fixtureAlertID, out.Alert.Id)
		require.Len(t, out.Events, 2)
		require.Equal(t, fixtureLogUIDs[0], out.Events[0].Uuid)
	})

	t.Run("text output renders alert detail and events table", func(t *testing.T) {
		t.Parallel()
		alert := alertFixture(t)
		events := []logmodels.AccessgraphStorageV1alphaEvent{
			eventFixture(fixtureLogUIDs[0], time.Minute),
		}
		ag := newAccessGraphTestClient(t, newGetAlertHandler(t, alert, []fetchAllLogsPage{{
			data: eventsAsMaps(t, events),
		}}, &getAlertRequests{}, 0, ""))

		c, buf := newDetectionsCommand(t, teleport.Text)
		c.detections.get.id = alert.Id.String()
		require.NoError(t, c.DetectionsGet(context.Background(), ag))

		out := buf.String()
		// Header lines from displayDetectionText.
		require.Contains(t, out, "ID:                "+fixtureAlertID.String())
		require.Contains(t, out, "Title:             Unusual privilege escalation")
		require.Contains(t, out, "Severity:          high")
		require.Contains(t, out, "Period:            "+fixtureStart.Format(time.RFC3339)+" → "+fixtureEnd.Format(time.RFC3339))
		require.Contains(t, out, "Affected Entity:   alice@example.com")
		// Events table.
		require.Contains(t, out, "Log Entries:")
		require.Contains(t, out, "alice")
		require.Contains(t, out, "prod-db")
		// Status-change table.
		require.Contains(t, out, "Status Changes:")
	})

	t.Run("text output omits optional headers when fields are nil", func(t *testing.T) {
		t.Parallel()
		alert := accessgraph.SecurityAlert{
			Id:        fixtureAlertID,
			Title:     "Bare alert",
			Type:      "anomaly",
			Severity:  accessgraph.SecurityAlertSeverity("low"),
			Status:    accessgraph.AlertStatus("open"),
			Source:    logmodels.EventSource("aws"),
			StartTime: fixtureStart,
			EndTime:   fixtureEnd,
			CreatedAt: fixtureCreated,
		}
		ag := newAccessGraphTestClient(t, newGetAlertHandler(t, alert, nil, &getAlertRequests{}, 0, ""))

		c, buf := newDetectionsCommand(t, teleport.Text)
		c.detections.get.id = alert.Id.String()
		require.NoError(t, c.DetectionsGet(context.Background(), ag))

		out := buf.String()
		require.Contains(t, out, "ID:                "+fixtureAlertID.String())
		require.Contains(t, out, "Title:             Bare alert")
		for _, header := range []string{
			"Reported By:", "Updated:", "Affected Entity:", "Tags:",
			"Description:", "Mitigation Steps:", "Log Entries:", "Status Changes:",
		} {
			require.NotContains(t, out, header, "optional section %q must not render for a minimal alert", header)
		}
	})

	t.Run("HTTP 404 from get surfaces as apiResponseError", func(t *testing.T) {
		t.Parallel()
		alert := alertFixture(t)
		ag := newAccessGraphTestClient(t, newGetAlertHandler(t, alert, nil, &getAlertRequests{},
			http.StatusNotFound, `{"message":"no such alert"}`))

		c, _ := newDetectionsCommand(t, teleport.JSON)
		c.detections.get.id = alert.Id.String()
		err := c.DetectionsGet(context.Background(), ag)
		var agErr *apiResponseError
		require.ErrorAs(t, err, &agErr)
		require.Equal(t, http.StatusNotFound, agErr.StatusCode)
		require.Equal(t, "no such alert", agErr.Message)
	})
}

func TestDisplayDetectionTextAffectedEntity(t *testing.T) {
	t.Parallel()

	strPtr := func(s string) *string { return &s }

	cases := []struct {
		name     string
		entName  *string
		wantLine string // empty = no Affected line
	}{
		{"name set", strPtr("alice@example.com"), "Affected Entity:   alice@example.com"},
		{"name nil", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			alert := accessgraph.SecurityAlert{
				Id: fixtureAlertID, Title: "x", Type: "x",
				Severity:  accessgraph.SecurityAlertSeverity("low"),
				Status:    accessgraph.AlertStatus("open"),
				Source:    logmodels.EventSource("aws"),
				StartTime: fixtureStart, EndTime: fixtureEnd, CreatedAt: fixtureCreated,
				AffectedEntity: &struct {
					Name *string `json:"name,omitempty"`
					Type *string `json:"type,omitempty"`
				}{Name: tc.entName},
			}
			var buf bytes.Buffer
			require.NoError(t, displayDetectionText(&buf, alert, nil, nil))
			out := buf.String()
			if tc.wantLine == "" {
				require.NotContains(t, out, "Affected Entity:")
				return
			}
			require.Contains(t, out, tc.wantLine)
		})
	}
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

	t.Run("get requires id argument", func(t *testing.T) {
		t.Parallel()
		_, err := parse(t, "detections", "get")
		require.Error(t, err)
	})

	t.Run("get captures id positional", func(t *testing.T) {
		t.Parallel()
		got, err := parse(t, "detections", "get", fixtureAlertID.String())
		require.NoError(t, err)
		require.Equal(t, fixtureAlertID.String(), got.get.id)
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

	t.Run("non-advancing cursor breaks the loop", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int64
		// Always returns the same non-nil cursor regardless of input, which
		// would loop forever without the cursor-advance guard.
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":        []accessgraph.SecurityAlert{alertFixture(t)},
				"next_cursor": "stuck",
			})
		})
		ag := newAccessGraphTestClient(t, handler)
		got, err := fetchAlerts(context.Background(), ag, accessgraph.ListAlertsV1Params{}, 0)
		require.NoError(t, err)
		require.Len(t, got, 2, "should return alerts collected before the loop broke")
		require.EqualValues(t, 2, calls.Load(), "loop must stop on the first non-advancing cursor")
	})
}

// eventsAsMaps re-marshals events through JSON so the handler's
// fetchAllLogsPage (which carries []map[string]any) can serialize them with
// the exact field shape the AG schema expects.
func eventsAsMaps(t *testing.T, events []logmodels.AccessgraphStorageV1alphaEvent) []map[string]any {
	t.Helper()
	out := make([]map[string]any, 0, len(events))
	for _, ev := range events {
		b, err := json.Marshal(ev)
		require.NoError(t, err)
		var m map[string]any
		require.NoError(t, json.Unmarshal(b, &m))
		out = append(out, m)
	}
	return out
}
