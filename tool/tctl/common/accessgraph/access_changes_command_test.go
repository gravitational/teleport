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
	"sync/atomic"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
)

// AG-mounted paths the access-changes commands hit — kept as constants so
// the test fails loudly if the generated client's operation paths drift.
const (
	listAccessChangesPath = accessGraphAPIPath + "graph/crown-jewel/access-paths"
)

var (
	fixtureChangeID   = "ch-1111"
	fixtureAffectedID = uuid.MustParse("66666666-7777-8888-9999-000000000000")
	fixtureChangeTime = time.Date(2026, 5, 1, 9, 30, 0, 0, time.UTC)
)

func accessPathSummaryItemFixture() accessgraph.AccessPathSummaryItem {
	return accessgraph.AccessPathSummaryItem{
		Id:        fixtureChangeID,
		CreatedAt: fixtureChangeTime,
		AffectedNode: accessgraph.AccessPathSummaryItemNode{
			Id:         fixtureAffectedID,
			Kind:       "resource",
			Name:       "prod-db",
			Source:     "Teleport",
			OriginType: "teleport_database",
			Alias:      "prod-db-alias",
		},
	}
}

type accessChangesRequest struct {
	path  string
	query url.Values
}

func newListAccessChangesHandler(t *testing.T, items []accessgraph.AccessPathSummaryItem, captured *accessChangesRequest, statusCode int, errBody string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if captured != nil {
			*captured = accessChangesRequest{path: r.URL.Path, query: r.URL.Query()}
		}
		require.Equal(t, listAccessChangesPath, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if statusCode != 0 && statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
			_, _ = w.Write([]byte(errBody))
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
	})
}

type fetchAccessChangesPage struct {
	data       []accessgraph.AccessPathSummaryItem
	nextCursor *string
}

// newPaginatedAccessChangesHandler advances through pages on each request,
// asserting the inbound iterator matches the prior page's cursor.
func newPaginatedAccessChangesHandler(t *testing.T, pages []fetchAccessChangesPage) http.Handler {
	t.Helper()
	var calls atomic.Int64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(calls.Add(1) - 1)
		require.Less(t, idx, len(pages), "more page requests than configured")
		if idx > 0 {
			prev := pages[idx-1].nextCursor
			require.NotNil(t, prev, "fetchAccessChanges continued past a nil cursor")
			require.Equal(t, *prev, r.URL.Query().Get("iterator"))
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

// newAccessChangesCommand returns a command wired to a captured stdout
// buffer. Tests call AccessChangesList/AccessChangesGet directly — TryRun
// is bypassed because it owns credential loading, not behavior.
func newAccessChangesCommand(t *testing.T, format string) (*AccessGraphCommand, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	c := &AccessGraphCommand{
		stdout:        &buf,
		accessChanges: accessChangesArgs{format: format},
	}
	return c, &buf
}

func TestAccessChangesList(t *testing.T) {

	t.Run("request shaping carries filter envelopes and search", func(t *testing.T) {
		var got accessChangesRequest
		ag := newAccessGraphTestClient(t, newListAccessChangesHandler(t,
			[]accessgraph.AccessPathSummaryItem{accessPathSummaryItemFixture()}, &got, 0, ""))

		c, _ := newAccessChangesCommand(t, teleport.JSON)
		c.accessChanges.ls = accessChangesListArgs{
			search:  "prod-db",
			filters: []string{"kind=resource,source=TELEPORT", "type=teleport_user"},
			limit:   10,
		}
		require.NoError(t, c.AccessChangesList(context.Background(), ag))

		require.Equal(t, "prod-db", got.query.Get("search"))

		// type_filter is a JSON array with one envelope per --filter; the order
		// is preserved and within-envelope pairs are AND'd.
		var filters []accessChangeTypeFilter
		require.NoError(t, json.Unmarshal([]byte(got.query.Get("type_filter")), &filters))
		require.Len(t, filters, 2)
		require.NotNil(t, filters[0].Kind)
		require.Equal(t, "resource", *filters[0].Kind)
		require.NotNil(t, filters[0].Source)
		require.Equal(t, "TELEPORT", *filters[0].Source)
		require.Nil(t, filters[0].OriginType)
		require.NotNil(t, filters[1].OriginType)
		require.Equal(t, "teleport_user", *filters[1].OriginType)

		// constructAccessChangesListQuery deliberately leaves the per-page
		// Limit unset — the user --limit only trims via fetchAccessChanges.
		require.Empty(t, got.query.Get("limit"), "params.Limit must not be sent; server picks the page size")
	})

	t.Run("text format renders headers and row data", func(t *testing.T) {
		ag := newAccessGraphTestClient(t, newListAccessChangesHandler(t,
			[]accessgraph.AccessPathSummaryItem{accessPathSummaryItemFixture()}, nil, 0, ""))

		c, buf := newAccessChangesCommand(t, teleport.Text)
		require.NoError(t, c.AccessChangesList(context.Background(), ag))

		out := buf.String()
		for _, h := range []string{"Change ID", "Kind", "Name", "Source", "Origin Type", "Alias", "Created At"} {
			require.Contains(t, out, h, "missing column header %q", h)
		}
		require.Contains(t, out, fixtureChangeID)
		require.Contains(t, out, "prod-db")
		require.Contains(t, out, "Teleport")
	})

	t.Run("nil data renders as empty JSON array", func(t *testing.T) {
		// Handler returns {"data": null} — displayAccessChanges normalizes
		// the nil to [] so JSON output is a valid empty array.
		ag := newAccessGraphTestClient(t, newListAccessChangesHandler(t, nil, nil, 0, ""))

		c, buf := newAccessChangesCommand(t, teleport.JSON)
		require.NoError(t, c.AccessChangesList(context.Background(), ag))
		require.JSONEq(t, "[]", buf.String())
	})

	t.Run("HTTP 500 surfaces as apiResponseError", func(t *testing.T) {
		ag := newAccessGraphTestClient(t, newListAccessChangesHandler(t, nil, nil,
			http.StatusInternalServerError, `{"message":"changes backend exploded"}`))

		c, _ := newAccessChangesCommand(t, teleport.JSON)
		err := c.AccessChangesList(context.Background(), ag)
		var agErr *apiResponseError
		require.ErrorAs(t, err, &agErr)
		require.Equal(t, http.StatusInternalServerError, agErr.StatusCode)
		require.Equal(t, "changes backend exploded", agErr.Message)
	})

}

func TestInitAccessChangesFlags(t *testing.T) {

	parse := func(t *testing.T, argv ...string) (accessChangesArgs, error) {
		t.Helper()
		app := kingpin.New("tctl-test", "")
		c := &AccessGraphCommand{}
		c.initAccessChanges(app)
		_, err := app.Parse(argv)
		return c.accessChanges, err
	}

	t.Run("ls defaults", func(t *testing.T) {
		got, err := parse(t, "access-changes", "ls")
		require.NoError(t, err)
		require.Equal(t, teleport.Text, got.format)
		require.Equal(t, 100, got.ls.limit)
		require.Empty(t, got.ls.search)
		require.Empty(t, got.ls.filters)
	})

	t.Run("ls collects repeated --filter flags in order", func(t *testing.T) {
		got, err := parse(t, "access-changes", "ls",
			"--search", "prod",
			"--filter", "kind=resource,source=AWS",
			"--filter", "type=teleport_user",
			"--limit", "250",
		)
		require.NoError(t, err)
		require.Equal(t, "prod", got.ls.search)
		require.Equal(t, []string{"kind=resource,source=AWS", "type=teleport_user"}, got.ls.filters)
		require.Equal(t, 250, got.ls.limit)
	})

	t.Run("ls collects repeated dedicated flags in order", func(t *testing.T) {
		got, err := parse(t, "access-changes", "ls",
			"--type", "teleport_user",
			"--type", "aws_s3",
			"--kind", "identity",
			"--source", "AWS",
		)
		require.NoError(t, err)
		require.Equal(t, []string{"teleport_user", "aws_s3"}, got.ls.typ)
		require.Equal(t, []string{"identity"}, got.ls.kind)
		require.Equal(t, []string{"AWS"}, got.ls.source)
	})

	t.Run("ls rejects out-of-enum flag values", func(t *testing.T) {
		_, err := parse(t, "access-changes", "ls", "--kind", "bogus")
		require.Error(t, err)
	})
}

func TestFetchAccessChanges(t *testing.T) {

	t.Run("multi-page response walks the cursor", func(t *testing.T) {
		cursor := "page2"
		ag := newAccessGraphTestClient(t, newPaginatedAccessChangesHandler(t, []fetchAccessChangesPage{
			{data: []accessgraph.AccessPathSummaryItem{accessPathSummaryItemFixture()}, nextCursor: &cursor},
			{data: []accessgraph.AccessPathSummaryItem{accessPathSummaryItemFixture()}},
		}))
		got, err := fetchAccessChanges(context.Background(), ag, accessgraph.ListCrownJewelAccessPathsParams{}, 100)
		require.NoError(t, err)
		require.Len(t, got, 2)
	})

	t.Run("limit caps and trims across pages", func(t *testing.T) {
		cursor := "page2"
		page1 := []accessgraph.AccessPathSummaryItem{accessPathSummaryItemFixture(), accessPathSummaryItemFixture(), accessPathSummaryItemFixture()}
		page2 := []accessgraph.AccessPathSummaryItem{accessPathSummaryItemFixture(), accessPathSummaryItemFixture(), accessPathSummaryItemFixture()}
		ag := newAccessGraphTestClient(t, newPaginatedAccessChangesHandler(t, []fetchAccessChangesPage{
			{data: page1, nextCursor: &cursor},
			{data: page2},
		}))
		got, err := fetchAccessChanges(context.Background(), ag, accessgraph.ListCrownJewelAccessPathsParams{}, 4)
		require.NoError(t, err)
		require.Len(t, got, 4)
	})

	t.Run("limit zero disables the cap", func(t *testing.T) {
		cursor := "page2"
		ag := newAccessGraphTestClient(t, newPaginatedAccessChangesHandler(t, []fetchAccessChangesPage{
			{data: []accessgraph.AccessPathSummaryItem{accessPathSummaryItemFixture(), accessPathSummaryItemFixture()}, nextCursor: &cursor},
			{data: []accessgraph.AccessPathSummaryItem{accessPathSummaryItemFixture()}},
		}))
		got, err := fetchAccessChanges(context.Background(), ag, accessgraph.ListCrownJewelAccessPathsParams{}, 0)
		require.NoError(t, err)
		require.Len(t, got, 3)
	})

	t.Run("non-advancing cursor breaks the loop", func(t *testing.T) {
		var calls atomic.Int64
		// Always returns the same non-nil cursor regardless of input.
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":        []accessgraph.AccessPathSummaryItem{accessPathSummaryItemFixture()},
				"next_cursor": "stuck",
			})
		})
		ag := newAccessGraphTestClient(t, handler)
		got, err := fetchAccessChanges(context.Background(), ag, accessgraph.ListCrownJewelAccessPathsParams{}, 0)
		require.NoError(t, err)
		require.Len(t, got, 2, "should return items collected before the loop broke")
		require.EqualValues(t, 2, calls.Load(), "loop must stop on the first non-advancing cursor")
	})
}

func TestConstructAccessChangesListQuery(t *testing.T) {

	t.Run("no filters leaves TypeFilter and Search nil", func(t *testing.T) {
		params, err := constructAccessChangesListQuery(accessChangesArgs{})
		require.NoError(t, err)
		require.Nil(t, params.TypeFilter)
		require.Nil(t, params.Search)
	})

	t.Run("single key populates only that axis in the envelope", func(t *testing.T) {
		args := accessChangesArgs{}
		args.ls.filters = []string{"kind=resource"}
		params, err := constructAccessChangesListQuery(args)
		require.NoError(t, err)
		require.NotNil(t, params.TypeFilter)

		var filters []accessChangeTypeFilter
		require.NoError(t, json.Unmarshal([]byte(*params.TypeFilter), &filters))
		require.Len(t, filters, 1)
		require.NotNil(t, filters[0].Kind)
		require.Equal(t, "resource", *filters[0].Kind)
		// Unset axes drop out of the JSON envelope (omitempty + nil pointers).
		require.Nil(t, filters[0].OriginType)
		require.Nil(t, filters[0].Source)

		// Pin the JSON shape too — use the "key": form to avoid matching
		// "resource" as a substring of "source".
		require.NotContains(t, *params.TypeFilter, `"type":`)
		require.NotContains(t, *params.TypeFilter, `"source":`)
	})

	t.Run("kind and source AND into one envelope", func(t *testing.T) {
		args := accessChangesArgs{}
		args.ls.filters = []string{"kind=resource,source=TELEPORT"}
		params, err := constructAccessChangesListQuery(args)
		require.NoError(t, err)
		require.NotNil(t, params.TypeFilter)

		var filters []accessChangeTypeFilter
		require.NoError(t, json.Unmarshal([]byte(*params.TypeFilter), &filters))
		require.Len(t, filters, 1)
		require.Equal(t, "resource", *filters[0].Kind)
		require.Equal(t, "TELEPORT", *filters[0].Source)
		require.Nil(t, filters[0].OriginType)
	})

	t.Run("repeated filters OR into separate envelopes in order", func(t *testing.T) {
		args := accessChangesArgs{}
		args.ls.filters = []string{"type=teleport_bot", "kind=resource,source=AWS"}
		params, err := constructAccessChangesListQuery(args)
		require.NoError(t, err)
		require.NotNil(t, params.TypeFilter)

		var filters []accessChangeTypeFilter
		require.NoError(t, json.Unmarshal([]byte(*params.TypeFilter), &filters))
		require.Len(t, filters, 2)
		require.Equal(t, "teleport_bot", *filters[0].OriginType)
		require.Equal(t, "resource", *filters[1].Kind)
		require.Equal(t, "AWS", *filters[1].Source)
	})

	t.Run("dedicated --type produces a single origin_type envelope", func(t *testing.T) {
		args := accessChangesArgs{}
		args.ls.typ = []string{"teleport_database"}
		params, err := constructAccessChangesListQuery(args)
		require.NoError(t, err)
		require.NotNil(t, params.TypeFilter)

		var filters []accessChangeTypeFilter
		require.NoError(t, json.Unmarshal([]byte(*params.TypeFilter), &filters))
		require.Len(t, filters, 1)
		require.Equal(t, "teleport_database", *filters[0].OriginType)
		require.Nil(t, filters[0].Kind)
		require.Nil(t, filters[0].Source)
	})

	t.Run("dedicated --kind and --source never combine into one envelope", func(t *testing.T) {
		args := accessChangesArgs{}
		args.ls.kind = []string{"identity"}
		args.ls.source = []string{"AWS"}
		params, err := constructAccessChangesListQuery(args)
		require.NoError(t, err)
		require.NotNil(t, params.TypeFilter)

		var filters []accessChangeTypeFilter
		require.NoError(t, json.Unmarshal([]byte(*params.TypeFilter), &filters))
		require.Len(t, filters, 2)
		// Assembly order places kind envelopes before source envelopes.
		require.Equal(t, "identity", *filters[0].Kind)
		require.Nil(t, filters[0].Source)
		require.Equal(t, "AWS", *filters[1].Source)
		require.Nil(t, filters[1].Kind)
	})

	t.Run("repeated dedicated --type flags OR into separate envelopes", func(t *testing.T) {
		args := accessChangesArgs{}
		args.ls.typ = []string{"teleport_user", "aws_s3"}
		params, err := constructAccessChangesListQuery(args)
		require.NoError(t, err)

		var filters []accessChangeTypeFilter
		require.NoError(t, json.Unmarshal([]byte(*params.TypeFilter), &filters))
		require.Len(t, filters, 2)
		require.Equal(t, "teleport_user", *filters[0].OriginType)
		require.Equal(t, "aws_s3", *filters[1].OriginType)
	})

	t.Run("mixed --filter and dedicated flags assemble filters, type, kind, source", func(t *testing.T) {
		args := accessChangesArgs{}
		args.ls.filters = []string{"kind=resource,source=AWS"}
		args.ls.typ = []string{"teleport_user"}
		args.ls.kind = []string{"identity"}
		args.ls.source = []string{"Okta"}
		params, err := constructAccessChangesListQuery(args)
		require.NoError(t, err)

		var filters []accessChangeTypeFilter
		require.NoError(t, json.Unmarshal([]byte(*params.TypeFilter), &filters))
		require.Len(t, filters, 4)
		// 0: the --filter envelope (AND'd pair).
		require.Equal(t, "resource", *filters[0].Kind)
		require.Equal(t, "AWS", *filters[0].Source)
		// 1: --type, 2: --kind, 3: --source.
		require.Equal(t, "teleport_user", *filters[1].OriginType)
		require.Equal(t, "identity", *filters[2].Kind)
		require.Equal(t, "Okta", *filters[3].Source)
	})

	t.Run("invalid filters surface BadParameter", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			filter string
		}{
			{"unknown key", "color=blue"},
			{"missing value", "kind="},
			{"missing key", "=resource"},
			{"no equals", "resource"},
			{"invalid type value", "type=not_a_type"},
			{"invalid kind value", "kind=bogus"},
			{"invalid source value", "source=NotReal"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				args := accessChangesArgs{}
				args.ls.filters = []string{tc.filter}
				_, err := constructAccessChangesListQuery(args)
				require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
			})
		}
	})
}
