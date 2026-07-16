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
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
)

// identityAccessPath pins the AG path so a generated-client drift fails the test.
const identityAccessPath = accessGraphAPIPath + "graph/access/v1"

func TestBuildAccessReviewOutput(t *testing.T) {
	idID := uuid.New()
	resID := uuid.New()
	grStanding := uuid.New()
	grRequest := uuid.New()
	lastAccess := time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC)

	nodes := []accessgraph.IdentityAccessNode{
		{Id: idID, Name: "alice@corp", Kind: "identity", SubKind: new("user")},
		{Id: resID, Name: "prod-db", Kind: "resource", SubKind: new("db"), Alias: new("Production DB")},
		{Id: grStanding, Name: "admins", Kind: "identity_group", SubKind: new("role")},
		{Id: grRequest, Name: "oncall", Kind: "identity_group", SubKind: new("access_request"), Temporary: new(true)},
	}
	resp := &accessgraph.IdentityAccessResponse{
		Nodes: nodes,
		Data: []accessgraph.IdentityAccessRow{{
			Identity: idID,
			Resources: []accessgraph.IdentityAccessResource{{
				Resource: resID,
				AccessInfo: accessgraph.IdentityAccessDecision{
					Level:         accessgraph.IdentityAccessDecisionLevelStanding,
					Temporary:     new(true),
					GrantorCounts: accessgraph.IdentityAccessGrantorCounts{Standing: 1, Request: 1},
					Grantors: []accessgraph.IdentityAccessGrantor{
						{Id: grRequest, Level: accessgraph.IdentityAccessGrantorLevelRequest},
						{Id: grStanding, Level: accessgraph.IdentityAccessGrantorLevelStanding},
					},
					Activity: &accessgraph.IdentityAccessActivity{Count: 14, LastAccess: &lastAccess},
				},
			}},
		}},
	}

	t.Run("resolves nodes and access info", func(t *testing.T) {
		out := buildAccessReviewOutput(resp)
		require.Len(t, out.Identities, 1)

		ia := out.Identities[0]
		require.Equal(t, "alice@corp", ia.Identity.Name)
		require.Equal(t, "user", ia.Identity.SubKind)
		require.Len(t, ia.Resources, 1)

		ra := ia.Resources[0]
		require.Equal(t, "prod-db", ra.Resource.Name)
		require.Equal(t, "Production DB", ra.Resource.Alias)
		require.Equal(t, "standing", ra.Level)
		require.True(t, ra.Temporary)
		require.Equal(t, grantorCounts{Standing: 1, Request: 1}, ra.GrantorCounts)
		require.Len(t, ra.Grantors, 2)
		require.NotNil(t, ra.Activity)
		require.EqualValues(t, 14, ra.Activity.Count)
		require.Equal(t, &lastAccess, ra.Activity.LastAccess)
	})

	t.Run("primary grantor is the first grantor", func(t *testing.T) {
		out := buildAccessReviewOutput(resp)
		// The backend lists the primary grantor first, so index 0 is primary
		// regardless of the resolved level.
		g, ok := primaryGrantor(out.Identities[0].Resources[0])
		require.True(t, ok)
		require.Equal(t, "oncall", g.Node.Name)
		require.True(t, g.Node.Temporary)
	})

	t.Run("grantor temporary propagates from node", func(t *testing.T) {
		out := buildAccessReviewOutput(resp)
		grantors := out.Identities[0].Resources[0].Grantors
		require.Equal(t, "oncall", grantors[0].Node.Name)
		require.True(t, grantors[0].Node.Temporary)
	})

	t.Run("missing node tolerated", func(t *testing.T) {
		orphan := uuid.New()
		r := &accessgraph.IdentityAccessResponse{
			Nodes: nil,
			Data: []accessgraph.IdentityAccessRow{{
				Identity: orphan,
				Resources: []accessgraph.IdentityAccessResource{{
					Resource:   orphan,
					AccessInfo: accessgraph.IdentityAccessDecision{Level: accessgraph.IdentityAccessDecisionLevelStanding},
				}},
			}},
		}
		out := buildAccessReviewOutput(r)
		require.Equal(t, orphan.String(), out.Identities[0].Identity.ID)
		require.Empty(t, out.Identities[0].Identity.Name)
	})

	t.Run("activity absent when not provided", func(t *testing.T) {
		r := &accessgraph.IdentityAccessResponse{
			Nodes: nodes,
			Data: []accessgraph.IdentityAccessRow{{
				Identity: idID,
				Resources: []accessgraph.IdentityAccessResource{{
					Resource:   resID,
					AccessInfo: accessgraph.IdentityAccessDecision{Level: accessgraph.IdentityAccessDecisionLevelStanding},
				}},
			}},
		}
		out := buildAccessReviewOutput(r)
		require.Nil(t, out.Identities[0].Resources[0].Activity)
	})
}

func TestPrimaryGrantor(t *testing.T) {
	g := func(name, level string) grantor {
		return grantor{Node: node{Name: name}, Level: level}
	}

	t.Run("returns the first grantor", func(t *testing.T) {
		// The primary is index 0 even when a later grantor matches the level.
		ra := resourceAccess{Level: "standing", Grantors: []grantor{g("req", "request"), g("std", "standing")}}
		got, ok := primaryGrantor(ra)
		require.True(t, ok)
		require.Equal(t, "req", got.Node.Name)
	})

	t.Run("none when no grantors", func(t *testing.T) {
		_, ok := primaryGrantor(resourceAccess{Level: "standing"})
		require.False(t, ok)
	})
}

func TestGrantorSummary(t *testing.T) {
	cases := []struct {
		name string
		in   grantorCounts
		want string
	}{
		{"empty", grantorCounts{}, ""},
		{"standing only", grantorCounts{Standing: 2}, "2 standing"},
		{"impersonate only", grantorCounts{Impersonate: 1}, "1 impersonate"},
		{"mixed in fixed order", grantorCounts{Standing: 1, Impersonate: 2, Request: 3}, "1 standing, 2 impersonate, 3 request"},
		{"zero levels omitted", grantorCounts{Standing: 1, Request: 1}, "1 standing, 1 request"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, grantorSummary(tc.in))
		})
	}
}

func TestDisplayAccessReviewText(t *testing.T) {
	last := time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC)
	output := accessReviewOutput{
		Identities: []identityAccess{{
			Identity: node{ID: "i1", Name: "alice@corp", SubKind: "user"},
			Resources: []resourceAccess{
				{
					Resource:      node{Name: "prod-db", SubKind: "db"},
					Level:         "standing",
					GrantorCounts: grantorCounts{Standing: 1, Request: 1},
					Grantors: []grantor{
						{Node: node{Name: "admins"}, Level: "standing"},
						{Node: node{Name: "break-glass", Temporary: true}, Level: "request"},
					},
					Activity: &activity{Count: 14, LastAccess: &last},
				},
				{
					Resource:      node{Name: "prod-web", SubKind: "app"},
					Level:         "request",
					Temporary:     true,
					GrantorCounts: grantorCounts{Request: 1},
					Grantors:      []grantor{{Node: node{Name: "oncall"}, Level: "request"}},
				},
			},
		}},
	}

	t.Run("summary without window omits activity columns", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, displayAccessReviewText(&buf, output, time.Time{}, time.Time{}, false, false))
		out := buf.String()
		require.NotContains(t, out, "Last Access")
		require.NotContains(t, out, "Accesses")
		require.NotContains(t, out, "Period:")
		require.Contains(t, out, "alice@corp")
		require.Contains(t, out, "request*", "temporary access should be marked")
		require.Contains(t, out, "Resource Kind")
		require.Contains(t, out, "db")
		require.Contains(t, out, "app")
		require.NotContains(t, out, "break-glass", "summary shows only the primary grantor")
	})

	t.Run("summary with window shows activity and period", func(t *testing.T) {
		from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
		var buf bytes.Buffer
		require.NoError(t, displayAccessReviewText(&buf, output, from, to, true, false))
		out := buf.String()
		require.Contains(t, out, "Period:")
		require.Contains(t, out, "Last Access")
		require.Contains(t, out, "14")
		require.Contains(t, out, "never", "resource without activity shows never")
	})

	t.Run("identity cell blanked after first resource row", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, displayAccessReviewText(&buf, output, time.Time{}, time.Time{}, false, false))
		// alice@corp must appear exactly once even though she has two resources.
		require.Equal(t, 1, bytes.Count(buf.Bytes(), []byte("alice@corp")))
	})

	t.Run("temporary primary grantor is marked", func(t *testing.T) {
		o := accessReviewOutput{Identities: []identityAccess{{
			Identity: node{Name: "bob@corp", SubKind: "user"},
			Resources: []resourceAccess{{
				Resource: node{Name: "vault", SubKind: "db"},
				Level:    "request",
				Grantors: []grantor{{Node: node{Name: "break-glass", Temporary: true}, Level: "request"}},
			}},
		}}}
		var buf bytes.Buffer
		require.NoError(t, displayAccessReviewText(&buf, o, time.Time{}, time.Time{}, false, false))
		require.Contains(t, buf.String(), "break-glass*", "a temporary primary grantor should be marked")
	})

	t.Run("legend keys the marker in both views", func(t *testing.T) {
		const legend = "* marks self-expiring access or a temporary grantor"
		var summary, detailed bytes.Buffer
		require.NoError(t, displayAccessReviewText(&summary, output, time.Time{}, time.Time{}, false, false))
		require.NoError(t, displayAccessReviewText(&detailed, output, time.Time{}, time.Time{}, false, true))
		require.Contains(t, summary.String(), legend)
		require.Contains(t, detailed.String(), legend)
	})

	t.Run("empty result", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, displayAccessReviewText(&buf, accessReviewOutput{}, time.Time{}, time.Time{}, false, false))
		require.Contains(t, buf.String(), "No access found.")
		require.NotContains(t, buf.String(), "* marks", "no legend without a table")
	})

	t.Run("warnings printed", func(t *testing.T) {
		var buf bytes.Buffer
		o := accessReviewOutput{Warnings: []string{"activity unavailable: boom"}}
		require.NoError(t, displayAccessReviewText(&buf, o, time.Time{}, time.Time{}, false, false))
		require.Contains(t, buf.String(), "Warning: activity unavailable: boom")
	})
}

// TestBuildAccessTable pins the layout policy: every column renders in full
// when the table fits the terminal, and only the Resource column is ever
// bounded — so naturally wide columns like Grantor Counts and Last Access are never
// truncated just because the table has many columns.
func TestBuildAccessTable(t *testing.T) {
	headers := []string{"Identity", "Kind", "Resource", "Resource Kind", "Access Level", "Grantor", "Grantor Counts", "Accesses", "Last Access"}
	rows := [][]string{
		{"ghassan", "user", "teleport-mcp-demo", "app", "standing", "Local DB ACL", "3 standing, 1 request", "6", "2026-06-05T19:15:32-07:00"},
	}

	t.Run("wide terminal renders every column in full", func(t *testing.T) {
		table := buildAccessTable(headers, rows, 200)
		out := table.String()
		require.NotContains(t, out, "...", "nothing should truncate when the table fits the terminal")
		require.Contains(t, out, "3 standing, 1 request")
		require.Contains(t, out, "2026-06-05T19:15:32-07:00")
		require.Contains(t, out, "teleport-mcp-demo")
	})

	t.Run("narrow terminal bounds only the Resource column", func(t *testing.T) {
		longResource := strings.Repeat("x", 80)
		narrowRows := [][]string{
			{"ghassan", "user", longResource, "app", "standing", "Local DB ACL", "3 standing, 1 request", "6", "2026-06-05T19:15:32-07:00"},
		}
		table := buildAccessTable(headers, narrowRows, 60)
		out := table.String()
		// The wide non-resource columns stay intact even on a narrow terminal...
		require.Contains(t, out, "3 standing, 1 request")
		require.Contains(t, out, "2026-06-05T19:15:32-07:00")
		// ...while the oversized Resource name is the only thing truncated.
		require.NotContains(t, out, longResource, "an oversized resource name should be bounded")
		require.Contains(t, out, "...")
	})
}

// accessPageHandler serves the configured response pages in order, recording
// the iterator the client sent on each call.
func accessPageHandler(t *testing.T, pages []accessgraph.IdentityAccessResponse) (http.Handler, *[]string) {
	t.Helper()
	var (
		calls     atomic.Int64
		iterators []string
	)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(calls.Add(1) - 1)
		require.Equal(t, identityAccessPath, r.URL.Path, "generated client drifted from the AG access path")
		iterators = append(iterators, r.URL.Query().Get("iterator"))
		require.Less(t, idx, len(pages), "client requested more pages than configured")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(pages[idx]))
	})
	return h, &iterators
}

func TestFetchIdentityAccess(t *testing.T) {
	row := func() accessgraph.IdentityAccessRow {
		return accessgraph.IdentityAccessRow{Identity: uuid.New()}
	}
	baseParams := accessgraph.ListIdentityAccessParams{Query: "SELECT * FROM access_path"}

	t.Run("single page, no cursor", func(t *testing.T) {
		h, iters := accessPageHandler(t, []accessgraph.IdentityAccessResponse{
			{Data: []accessgraph.IdentityAccessRow{row()}},
		})
		c := newAccessGraphTestClient(t, h)
		resp, truncated, err := fetchIdentityAccess(context.Background(), c, baseParams, 100)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Len(t, resp.Data, 1)
		require.Equal(t, []string{""}, *iters, "first call must omit the iterator")
	})

	t.Run("walks the cursor across pages and dedups nodes", func(t *testing.T) {
		shared := accessgraph.IdentityAccessNode{Id: uuid.New(), Name: "shared", Kind: "identity"}
		h, iters := accessPageHandler(t, []accessgraph.IdentityAccessResponse{
			{Data: []accessgraph.IdentityAccessRow{row()}, Nodes: []accessgraph.IdentityAccessNode{shared}, NextCursor: new("c1")},
			{Data: []accessgraph.IdentityAccessRow{row()}, Nodes: []accessgraph.IdentityAccessNode{shared}, NextCursor: new("c2")},
			{Data: []accessgraph.IdentityAccessRow{row()}},
		})
		c := newAccessGraphTestClient(t, h)
		resp, truncated, err := fetchIdentityAccess(context.Background(), c, baseParams, 100)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Len(t, resp.Data, 3)
		require.Len(t, resp.Nodes, 1, "shared node must be deduplicated")
		require.Equal(t, []string{"", "c1", "c2"}, *iters)
	})

	t.Run("truncates at maxResults", func(t *testing.T) {
		h, _ := accessPageHandler(t, []accessgraph.IdentityAccessResponse{
			{Data: []accessgraph.IdentityAccessRow{row(), row(), row()}, NextCursor: new("more")},
		})
		c := newAccessGraphTestClient(t, h)
		resp, truncated, err := fetchIdentityAccess(context.Background(), c, baseParams, 2)
		require.NoError(t, err)
		require.True(t, truncated)
		require.Len(t, resp.Data, 2)
	})

	t.Run("non-advancing cursor stops pagination", func(t *testing.T) {
		h, _ := accessPageHandler(t, []accessgraph.IdentityAccessResponse{
			{Data: []accessgraph.IdentityAccessRow{row()}, NextCursor: new("stuck")},
			{Data: []accessgraph.IdentityAccessRow{row()}, NextCursor: new("stuck")},
		})
		c := newAccessGraphTestClient(t, h)
		resp, truncated, err := fetchIdentityAccess(context.Background(), c, baseParams, 100)
		require.NoError(t, err)
		require.True(t, truncated)
		require.Len(t, resp.Data, 1)
	})

	t.Run("iac_error propagated", func(t *testing.T) {
		h, _ := accessPageHandler(t, []accessgraph.IdentityAccessResponse{
			{Data: []accessgraph.IdentityAccessRow{row()}, IacError: new("activity center down")},
		})
		c := newAccessGraphTestClient(t, h)
		resp, _, err := fetchIdentityAccess(context.Background(), c, baseParams, 100)
		require.NoError(t, err)
		require.NotNil(t, resp.IacError)
		require.Equal(t, "activity center down", *resp.IacError)
	})
}

func TestAccessReviewFlags(t *testing.T) {
	newApp := func() (*kingpin.Application, *AccessGraphCommand) {
		app := kingpin.New("tctl", "")
		app.Terminate(nil)
		c := &AccessGraphCommand{}
		c.initAccessReview(app)
		return app, c
	}

	t.Run("defaults", func(t *testing.T) {
		app, c := newApp()
		_, err := app.Parse([]string{"access-review", "--query", "SELECT * FROM access_path"})
		require.NoError(t, err)
		require.Equal(t, "SELECT * FROM access_path", c.accessReview.query)
		require.Equal(t, 50, c.accessReview.limit)
		require.Equal(t, teleport.Text, c.accessReview.format)
		require.False(t, c.accessReview.detailed)
		require.True(t, c.accessReview.from.IsZero())
		require.True(t, c.accessReview.to.IsZero())
	})

	t.Run("query required", func(t *testing.T) {
		app, _ := newApp()
		_, err := app.Parse([]string{"access-review"})
		require.Error(t, err)
	})
}

func TestAccessReviewValidation(t *testing.T) {
	// Returns an empty page so the happy path renders "No access found." without
	// the validation cases ever reaching the network.
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[],"nodes":[]}`))
	})
	client := newAccessGraphTestClient(t, h)

	run := func(args accessReviewArgs) (string, error) {
		var buf bytes.Buffer
		c := &AccessGraphCommand{stdout: &buf, accessReview: args}
		err := c.AccessReview(context.Background(), client)
		return buf.String(), err
	}
	base := accessReviewArgs{query: "SELECT * FROM access_path", limit: 50, format: teleport.Text}

	t.Run("limit below range", func(t *testing.T) {
		args := base
		args.limit = 0
		_, err := run(args)
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
	})

	t.Run("--to without --from", func(t *testing.T) {
		args := base
		args.to = time.Now()
		_, err := run(args)
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
	})

	t.Run("--from in the future is rejected", func(t *testing.T) {
		args := base
		args.from = time.Now().Add(time.Hour)
		_, err := run(args)
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
	})

	t.Run("valid no-window review renders", func(t *testing.T) {
		out, err := run(base)
		require.NoError(t, err)
		require.Contains(t, out, "No access found.")
	})
}

func TestAccessReviewSurfacesWarnings(t *testing.T) {
	run := func(t *testing.T, pages []accessgraph.IdentityAccessResponse, args accessReviewArgs) string {
		t.Helper()
		h, _ := accessPageHandler(t, pages)
		client := newAccessGraphTestClient(t, h)
		var buf bytes.Buffer
		c := &AccessGraphCommand{stdout: &buf, accessReview: args}
		require.NoError(t, c.AccessReview(context.Background(), client))
		return buf.String()
	}
	base := accessReviewArgs{query: "SELECT * FROM access_path", limit: 50, format: teleport.Text}

	t.Run("truncation warning", func(t *testing.T) {
		args := base
		args.limit = 1
		out := run(t, []accessgraph.IdentityAccessResponse{
			{Data: []accessgraph.IdentityAccessRow{{Identity: uuid.New()}, {Identity: uuid.New()}}, NextCursor: new("more")},
		}, args)
		require.Contains(t, out, "truncated at 1 identities")
	})

	t.Run("iac_error warning", func(t *testing.T) {
		out := run(t, []accessgraph.IdentityAccessResponse{
			{Data: []accessgraph.IdentityAccessRow{{Identity: uuid.New()}}, IacError: new("activity center down")},
		}, base)
		require.Contains(t, out, "activity unavailable: activity center down")
	})
}
