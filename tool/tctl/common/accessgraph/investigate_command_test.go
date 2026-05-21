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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	logmodels "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/logs"
)

// logsStatsQueryPath pins the AG path so a generated-client drift fails the test.
const logsStatsQueryPath = accessGraphAPIPath + "graph/logs/v1/stats"

// TestFilterFieldsUnique pins that no two entries share a flag or lucene name.
func TestFilterFieldsUnique(t *testing.T) {
	t.Parallel()
	a := &investigateArgs{}
	flags := map[string]bool{}
	lucene := map[string]bool{}
	for _, f := range a.filterFields() {
		require.False(t, flags[f.flag], "duplicate flag %q", f.flag)
		require.False(t, lucene[f.lucene], "duplicate lucene %q", f.lucene)
		flags[f.flag] = true
		lucene[f.lucene] = true
	}
}

func TestInvestigateBuildQuery(t *testing.T) {
	t.Parallel()

	t.Run("no filters → empty query", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{}
		require.Empty(t, a.buildQuery())
	})

	t.Run("raw query short-circuits structured assembly", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{
			rawQuery:        `identity_id:"alice"`,
			includeIdentity: []string{"bob"}, // ignored when rawQuery is set
		}
		require.Equal(t, `identity_id:"alice"`, a.buildQuery())
	})

	t.Run("single include emits field:value", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{includeIdentity: []string{"alice@example.com"}}
		require.Equal(t, `identity_id:"alice@example.com"`, a.buildQuery())
	})

	t.Run("multiple values on the same flag are OR'd", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{includeEventType: []string{"session.start", "session.end"}}
		require.Equal(t, `event_type:("session.start" OR "session.end")`, a.buildQuery())
	})

	t.Run("exclude is wrapped in NOT", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{excludeStatus: []string{"success"}}
		require.Equal(t, `NOT status:"success"`, a.buildQuery())
	})

	t.Run("include + exclude across fields → AND-joined", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{
			includeIdentity: []string{"alice"},
			excludeStatus:   []string{"success"},
		}
		// filterFields() order (identity before status) pins the full string.
		require.Equal(t, `identity_id:"alice" AND NOT status:"success"`, a.buildQuery())
	})
}

func TestInvestigateValidateRawQueryExclusive(t *testing.T) {
	t.Parallel()

	t.Run("no raw query → never errors", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{includeIdentity: []string{"alice"}}
		require.NoError(t, a.validateRawQueryExclusive())
	})

	t.Run("raw query alone is allowed", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{rawQuery: `status:"error"`}
		require.NoError(t, a.validateRawQueryExclusive())
	})

	t.Run("raw query plus include → BadParameter listing offender", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{
			rawQuery:        `status:"error"`,
			includeIdentity: []string{"alice"},
		}
		err := a.validateRawQueryExclusive()
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
		require.Contains(t, err.Error(), "--user")
	})

	t.Run("raw query plus exclude → BadParameter listing offender", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{
			rawQuery:      `status:"error"`,
			excludeStatus: []string{"success"},
		}
		err := a.validateRawQueryExclusive()
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
		require.Contains(t, err.Error(), "--exclude-status")
	})
}

func TestInvestigateValidateGeo(t *testing.T) {
	t.Parallel()

	f := func(v float32) *float32 { return &v }

	t.Run("none set → OK", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{}
		require.NoError(t, a.validateGeo())
	})

	t.Run("all three set → OK", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{latitude: f(37.7), longitude: f(-122.4), radius: f(10)}
		require.NoError(t, a.validateGeo())
	})

	t.Run("partial geo lists every missing flag", func(t *testing.T) {
		t.Parallel()
		a := &investigateArgs{latitude: f(37.7)}
		err := a.validateGeo()
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
		_, missing, _ := strings.Cut(err.Error(), "missing: ")
		require.Equal(t, "--longitude, --radius", missing)
	})
}

// TestStripUnmatchedFacets covers the count<0 sentinel: the backend uses
// count == -1 for values present in the window but absent from the filter.
func TestStripUnmatchedFacets(t *testing.T) {
	t.Parallel()

	t.Run("drops negative counts, keeps positive", func(t *testing.T) {
		t.Parallel()
		in := []logsFacet{{Name: "user", Values: []logsFacetValue{
			{Value: "alice", Count: 5},
			{Value: "bob", Count: -1},
			{Value: "carol", Count: 2},
		}}}
		out := stripUnmatchedFacets(in)
		require.Len(t, out, 1)
		require.Equal(t, []logsFacetValue{
			{Value: "alice", Count: 5},
			{Value: "carol", Count: 2},
		}, out[0].Values)
	})

	t.Run("facet with only unmatched values is dropped", func(t *testing.T) {
		t.Parallel()
		in := []logsFacet{
			{Name: "user", Values: []logsFacetValue{{Value: "alice", Count: 5}}},
			{Name: "user-agent", Values: []logsFacetValue{{Value: "ua1", Count: -1}}},
		}
		out := stripUnmatchedFacets(in)
		require.Len(t, out, 1, "user-agent facet should have been dropped")
		require.Equal(t, "user", out[0].Name)
	})
}

// statsResponse builds a stats endpoint body for table tests.
func statsResponse(columns ...statsColumn) map[string]any {
	data := make([]map[string]any, 0, len(columns))
	for _, c := range columns {
		values := make([]map[string]any, 0, len(c.values))
		for _, v := range c.values {
			values = append(values, map[string]any{"value": v.value, "count": v.count})
		}
		data = append(data, map[string]any{
			"column_name": c.name,
			"values":      values,
		})
	}
	return map[string]any{"data": data}
}

type statsColumn struct {
	name   string
	values []statsValue
}

type statsValue struct {
	value string
	count int64
}

func TestFetchLogsFacets(t *testing.T) {
	t.Parallel()

	a := &investigateArgs{}
	facetNames := a.facetNames()

	t.Run("renames + sorts + drops non-filterable columns", func(t *testing.T) {
		t.Parallel()
		body := statsResponse(
			// identity_id → "user", unsorted on the wire.
			statsColumn{name: "identity_id", values: []statsValue{
				{value: "bob", count: 1},
				{value: "alice", count: 7},
				{value: "carol", count: 3},
			}},
			// event_type drives the total and renders as "event-type".
			statsColumn{name: "event_type", values: []statsValue{
				{value: "session.start", count: 4},
				{value: "session.end", count: 5},
			}},
			// Non-filterable, must be dropped.
			statsColumn{name: "row_count", values: []statsValue{
				{value: "irrelevant", count: 99},
			}},
			// Empty values, must be dropped.
			statsColumn{name: "status", values: nil},
		)
		ag := newAccessGraphTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, logsStatsQueryPath, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		}))

		facets, total, err := fetchLogsFacets(context.Background(), ag,
			accessgraph.ExecuteLogsStatsQueryV1Params{}, facetNames)
		require.NoError(t, err)
		require.EqualValues(t, 9, total, "sum of event_type counts (4 + 5)")

		names := make([]string, len(facets))
		for i, f := range facets {
			names[i] = f.Name
		}
		require.Equal(t, []string{"event-type", "user"}, names)

		userFacet := facets[1]
		require.Equal(t, []logsFacetValue{
			{Value: "alice", Count: 7},
			{Value: "carol", Count: 3},
			{Value: "bob", Count: 1},
		}, userFacet.Values, "sorted by count desc")
	})

	t.Run("negative counts in event_type don't inflate total", func(t *testing.T) {
		t.Parallel()
		body := statsResponse(statsColumn{name: "event_type", values: []statsValue{
			{value: "session.start", count: 4},
			{value: "session.error", count: -1},
		}})
		ag := newAccessGraphTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		}))
		_, total, err := fetchLogsFacets(context.Background(), ag,
			accessgraph.ExecuteLogsStatsQueryV1Params{}, facetNames)
		require.NoError(t, err)
		require.EqualValues(t, 4, total)
	})
}

var (
	investigateFixtureFrom = time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	investigateFixtureTo   = time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
)

// newInvestigateCommand returns a command wired to a captured buffer. Tests
// call Investigate directly, bypassing TryRun's credential loading.
func newInvestigateCommand(t *testing.T, format string) (*AccessGraphCommand, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	c := &AccessGraphCommand{
		stdout: &buf,
		investigate: investigateArgs{
			format: format,
			from:   investigateFixtureFrom,
			to:     investigateFixtureTo,
			order:  string(accessgraph.Desc),
			limit:  100,
		},
	}
	return c, &buf
}

// investigateHandler serves the AG stats and logs endpoints; nil logsPages
// asserts the logs route is never hit.
type investigateHandler struct {
	stats         map[string]any
	statsCalls    atomic.Int64
	statsQuery    string
	statsStart    string
	statsEnd      string
	logsPages     []fetchAllLogsPage
	logsCalls     atomic.Int64
	logsQuery     string
	logsStart     string
	logsEnd       string
	logsOrder     string
	logsLatitude  string
	logsLongitude string
	logsRadius    string
}

func (h *investigateHandler) serve(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case logsStatsQueryPath:
			h.statsCalls.Add(1)
			h.statsQuery = r.URL.Query().Get("query")
			h.statsStart = r.URL.Query().Get("start_time")
			h.statsEnd = r.URL.Query().Get("end_time")
			_ = json.NewEncoder(w).Encode(h.stats)
		case logsQueryPath:
			idx := int(h.logsCalls.Add(1) - 1)
			if idx == 0 {
				h.logsQuery = r.URL.Query().Get("query")
				h.logsStart = r.URL.Query().Get("start_time")
				h.logsEnd = r.URL.Query().Get("end_time")
				h.logsOrder = r.URL.Query().Get("order")
				h.logsLatitude = r.URL.Query().Get("latitude")
				h.logsLongitude = r.URL.Query().Get("longitude")
				h.logsRadius = r.URL.Query().Get("radius")
			}
			require.Less(t, idx, len(h.logsPages), "more pages requested than configured")
			page := h.logsPages[idx]
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

func TestInvestigate(t *testing.T) {
	t.Parallel()

	t.Run("--skill prints the embedded skill and skips the backend", func(t *testing.T) {
		t.Parallel()
		ag := newAccessGraphTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("server reached despite --skill: %s", r.URL.Path)
		}))
		c, buf := newInvestigateCommand(t, teleport.Text)
		c.investigate.skill = true

		require.NoError(t, c.Investigate(context.Background(), ag))
		require.Contains(t, buf.String(), "tctl investigate")
	})

	t.Run("--print-query prints the assembled query and skips the backend", func(t *testing.T) {
		t.Parallel()
		ag := newAccessGraphTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("server reached despite --print-query: %s", r.URL.Path)
		}))
		c, buf := newInvestigateCommand(t, teleport.Text)
		c.investigate.includeIdentity = []string{"alice"}
		c.investigate.excludeStatus = []string{"success"}
		c.investigate.printQuery = true

		require.NoError(t, c.Investigate(context.Background(), ag))
		require.Equal(t, `identity_id:"alice" AND NOT status:"success"`,
			strings.TrimSpace(buf.String()))
	})

	t.Run("invalid time window rejected before backend is hit", func(t *testing.T) {
		t.Parallel()
		ag := newAccessGraphTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("server reached despite invalid window: %s", r.URL.Path)
		}))
		c, _ := newInvestigateCommand(t, teleport.JSON)
		c.investigate.from = investigateFixtureTo
		c.investigate.to = investigateFixtureFrom
		err := c.Investigate(context.Background(), ag)
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
	})

	t.Run("full flow sends UTC window and structured query to both endpoints", func(t *testing.T) {
		t.Parallel()
		h := &investigateHandler{
			stats: statsResponse(
				statsColumn{name: "event_type", values: []statsValue{{value: "session.start", count: 3}}},
				statsColumn{name: "identity_id", values: []statsValue{{value: "alice", count: 3}}},
			),
			logsPages: []fetchAllLogsPage{{data: []map[string]any{
				{"uuid": "11111111-1111-1111-1111-111111111111"},
			}}},
		}
		ag := newAccessGraphTestClient(t, h.serve(t))

		c, _ := newInvestigateCommand(t, teleport.JSON)
		// Pin the from/to to a non-UTC zone so we can prove .UTC() conversion happens.
		zone := time.FixedZone("UTC-7", -7*3600)
		c.investigate.from = investigateFixtureFrom.In(zone)
		c.investigate.to = investigateFixtureTo.In(zone)
		c.investigate.includeIdentity = []string{"alice"}

		require.NoError(t, c.Investigate(context.Background(), ag))

		require.EqualValues(t, 1, h.statsCalls.Load())
		require.EqualValues(t, 1, h.logsCalls.Load())
		require.Equal(t, `identity_id:"alice"`, h.statsQuery)
		require.Equal(t, `identity_id:"alice"`, h.logsQuery)

		// Stats reinterprets the literal as UTC, so a non-UTC time would
		// shift the stats window relative to the logs window.
		gotStart, err := time.Parse(time.RFC3339Nano, h.statsStart)
		require.NoError(t, err)
		require.Equal(t, time.UTC, gotStart.Location())
		require.True(t, gotStart.Equal(investigateFixtureFrom))

		gotLogsStart, err := time.Parse(time.RFC3339Nano, h.logsStart)
		require.NoError(t, err)
		require.Equal(t, time.UTC, gotLogsStart.Location())

		require.Equal(t, "desc", h.logsOrder)
		require.Empty(t, h.logsLatitude)
	})

	t.Run("--facets-only skips the logs endpoint and emits empty events array", func(t *testing.T) {
		t.Parallel()
		h := &investigateHandler{
			stats: statsResponse(
				statsColumn{name: "event_type", values: []statsValue{{value: "session.start", count: 2}}},
			),
		}
		ag := newAccessGraphTestClient(t, h.serve(t))

		c, buf := newInvestigateCommand(t, teleport.JSON)
		c.investigate.facetsOnly = true

		require.NoError(t, c.Investigate(context.Background(), ag))
		require.EqualValues(t, 1, h.statsCalls.Load())
		require.EqualValues(t, 0, h.logsCalls.Load())

		var out struct {
			Total int64             `json:"total"`
			Data  []json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
		require.EqualValues(t, 2, out.Total)
		require.Empty(t, out.Data)

		// "data" must encode as [], never null, so jq consumers can `.data[]`.
		compact := strings.Join(strings.Fields(buf.String()), "")
		require.Contains(t, compact, `"data":[]`)
		require.NotContains(t, compact, `"data":null`)
	})

	t.Run("--show-unmatched preserves count=-1 values", func(t *testing.T) {
		t.Parallel()
		h := &investigateHandler{
			stats: statsResponse(
				statsColumn{name: "event_type", values: []statsValue{{value: "session.start", count: 4}}},
				statsColumn{name: "identity_id", values: []statsValue{
					{value: "alice", count: 4},
					{value: "bob", count: -1},
				}},
			),
			logsPages: []fetchAllLogsPage{{data: nil}},
		}
		ag := newAccessGraphTestClient(t, h.serve(t))

		c, buf := newInvestigateCommand(t, teleport.JSON)
		c.investigate.showUnmatched = true
		require.NoError(t, c.Investigate(context.Background(), ag))

		var out investigateOutput
		require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
		var userFacet *logsFacet
		for i := range out.Facets {
			if out.Facets[i].Name == "user" {
				userFacet = &out.Facets[i]
			}
		}
		require.NotNil(t, userFacet)
		require.Len(t, userFacet.Values, 2)
		// Sorted by count desc, so unmatched (-1) trails the positive count.
		require.EqualValues(t, 4, userFacet.Values[0].Count)
		require.EqualValues(t, -1, userFacet.Values[1].Count)
	})

	t.Run("yaml format round-trips total + facets + data", func(t *testing.T) {
		t.Parallel()
		h := &investigateHandler{
			stats: statsResponse(
				statsColumn{name: "event_type", values: []statsValue{{value: "session.start", count: 1}}},
			),
			logsPages: []fetchAllLogsPage{{data: []map[string]any{
				{"uuid": "11111111-1111-1111-1111-111111111111"},
			}}},
		}
		ag := newAccessGraphTestClient(t, h.serve(t))

		c, buf := newInvestigateCommand(t, teleport.YAML)
		require.NoError(t, c.Investigate(context.Background(), ag))

		var out investigateOutput
		require.NoError(t, yaml.Unmarshal(buf.Bytes(), &out))
		require.EqualValues(t, 1, out.Total)
		require.Len(t, out.Data, 1)
	})

	t.Run("text format renders period, matches, facet panel, and events table", func(t *testing.T) {
		t.Parallel()
		h := &investigateHandler{
			stats: statsResponse(
				statsColumn{name: "event_type", values: []statsValue{{value: "session.start", count: 2}}},
				statsColumn{name: "identity_id", values: []statsValue{
					{value: "alice", count: 2},
				}},
			),
			logsPages: []fetchAllLogsPage{{data: eventsAsMaps(t, []logmodels.AccessgraphStorageV1alphaEvent{
				eventFixture(fixtureLogUIDs[0], time.Minute),
			})}},
		}
		ag := newAccessGraphTestClient(t, h.serve(t))

		c, buf := newInvestigateCommand(t, teleport.Text)
		require.NoError(t, c.Investigate(context.Background(), ag))
		out := buf.String()

		require.Contains(t, out, "Period: "+investigateFixtureFrom.Format(time.RFC3339))
		require.Contains(t, out, investigateFixtureTo.Format(time.RFC3339))
		require.Contains(t, out, "Matches: ~2 (showing 1)")
		require.Contains(t, out, "Facets:")
		require.Contains(t, out, "user")
		require.Contains(t, out, "alice (2)")
		require.Contains(t, out, "event-type")
		require.Contains(t, out, "Event Type")
		require.Contains(t, out, "authentication")
	})

	t.Run("text format with --facets-only omits the events suffix and table", func(t *testing.T) {
		t.Parallel()
		h := &investigateHandler{
			stats: statsResponse(
				statsColumn{name: "event_type", values: []statsValue{{value: "session.start", count: 3}}},
			),
		}
		ag := newAccessGraphTestClient(t, h.serve(t))

		c, buf := newInvestigateCommand(t, teleport.Text)
		c.investigate.facetsOnly = true
		require.NoError(t, c.Investigate(context.Background(), ag))
		out := buf.String()

		require.Contains(t, out, "Matches: ~3")
		require.NotContains(t, out, "showing")
		require.NotContains(t, out, "Event Type")
	})

	t.Run("text format on zero-facet result prints 'Facets: none'", func(t *testing.T) {
		t.Parallel()
		h := &investigateHandler{
			stats:     statsResponse(),
			logsPages: []fetchAllLogsPage{{data: nil}},
		}
		ag := newAccessGraphTestClient(t, h.serve(t))

		c, buf := newInvestigateCommand(t, teleport.Text)
		require.NoError(t, c.Investigate(context.Background(), ag))
		require.Contains(t, buf.String(), "Facets: none")
	})

	t.Run("stats HTTP 500 surfaces as apiResponseError", func(t *testing.T) {
		t.Parallel()
		ag := newAccessGraphTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case logsStatsQueryPath:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"stats kaput"}`))
			case logsQueryPath:
				// Logs runs in parallel with stats; race-tolerant empty response.
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":[]}`))
			}
		}))
		c, _ := newInvestigateCommand(t, teleport.JSON)
		err := c.Investigate(context.Background(), ag)
		var agErr *apiResponseError
		require.ErrorAs(t, err, &agErr)
		require.Equal(t, http.StatusInternalServerError, agErr.StatusCode)
		require.Equal(t, "stats kaput", agErr.Message)
	})

	t.Run("geo filter parameters are forwarded to the logs endpoint", func(t *testing.T) {
		t.Parallel()
		h := &investigateHandler{
			stats:     statsResponse(statsColumn{name: "event_type", values: []statsValue{{value: "x", count: 1}}}),
			logsPages: []fetchAllLogsPage{{data: nil}},
		}
		ag := newAccessGraphTestClient(t, h.serve(t))

		c, _ := newInvestigateCommand(t, teleport.JSON)
		lat, lon, rad := float32(37.7), float32(-122.4), float32(10)
		c.investigate.latitude = &lat
		c.investigate.longitude = &lon
		c.investigate.radius = &rad

		require.NoError(t, c.Investigate(context.Background(), ag))
		require.Equal(t, "37.7", h.logsLatitude)
		require.Equal(t, "-122.4", h.logsLongitude)
		require.Equal(t, "10", h.logsRadius)
	})
}

func TestWriteWrappedList(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, writeWrappedList(&buf, "user:  ", []string{"alice (1)", "bob (1)", "carol (1)"}, 20))
	require.Equal(t, "user:  alice (1),\n       bob (1),\n       carol (1)\n", buf.String())
}

// TestInitInvestigateFlags exercises kingpin wiring without going through TryRun.
func TestInitInvestigateFlags(t *testing.T) {
	t.Parallel()

	// parse builds a fresh app per subtest so parser state is isolated.
	parse := func(t *testing.T, argv ...string) (investigateArgs, error) {
		t.Helper()
		app := kingpin.New("tctl-test", "")
		c := &AccessGraphCommand{}
		c.initInvestigate(app)
		_, err := app.Parse(argv)
		return c.investigate, err
	}

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		before := time.Now()
		got, err := parse(t, "investigate")
		after := time.Now()
		require.NoError(t, err)

		require.Equal(t, teleport.Text, got.format)
		require.Equal(t, string(accessgraph.Desc), got.order)
		require.Equal(t, 100, got.limit)
		require.False(t, got.skill)
		require.False(t, got.facetsOnly)

		// --from defaults to "1d"; widened slack absorbs CI clock drift.
		require.WithinRange(t, got.from,
			before.Add(-24*time.Hour-5*time.Second),
			after.Add(-24*time.Hour+5*time.Second))
		require.WithinRange(t, got.to, before.Add(-5*time.Second), after.Add(5*time.Second))
	})

	t.Run("structured filters populate include/exclude slices", func(t *testing.T) {
		t.Parallel()
		got, err := parse(t,
			"investigate",
			"--user", "alice",
			"--user", "bob",
			"--exclude-status", "success",
			"--user-agent", "Mozilla/5.0",
		)
		require.NoError(t, err)
		require.Equal(t, []string{"alice", "bob"}, got.includeIdentity)
		require.Equal(t, []string{"success"}, got.excludeStatus)
		require.Equal(t, []string{"Mozilla/5.0"}, got.includeUserAgent)
	})

	t.Run("source enum rejects unknown value", func(t *testing.T) {
		t.Parallel()
		_, err := parse(t, "investigate", "--source", "datadog")
		require.Error(t, err)
		require.Contains(t, err.Error(), "enum value must be one of")
	})

	t.Run("geo flags populate pointers", func(t *testing.T) {
		t.Parallel()
		// `--longitude=-122.4` so kingpin doesn't read `-122.4` as a flag.
		got, err := parse(t, "investigate",
			"--latitude", "37.7",
			"--longitude=-122.4",
			"--radius", "10",
		)
		require.NoError(t, err)
		require.NotNil(t, got.latitude)
		require.NotNil(t, got.longitude)
		require.NotNil(t, got.radius)
		require.InDelta(t, 37.7, *got.latitude, 1e-4)
		require.InDelta(t, -122.4, *got.longitude, 1e-4)
		require.InDelta(t, 10, *got.radius, 1e-9)
	})

	t.Run("--query and --skill toggles parse without value", func(t *testing.T) {
		t.Parallel()
		got, err := parse(t, "investigate", "--skill", "--query", `status:"error"`)
		require.NoError(t, err)
		require.True(t, got.skill)
		require.Equal(t, `status:"error"`, got.rawQuery)
	})
}
