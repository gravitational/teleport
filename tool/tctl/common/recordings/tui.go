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

// Package recordings provides an interactive TUI for browsing session
// recording search results returned by tctl recordings search.
package recordings

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	sessionsearchv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

// SummaryGetter fetches the AI-generated summary for a recorded session.
// [summarizerv1pb.SummarizerServiceClient] satisfies this interface.
type SummaryGetter interface {
	GetSummary(context.Context, *summarizerv1pb.GetSummaryRequest, ...grpc.CallOption) (*summarizerv1pb.GetSummaryResponse, error)
}

// BatchFetcher fetches the next page of sessions using a continuation token.
// Returns the sessions, the next token (empty = no more pages), and any error.
type BatchFetcher func(ctx context.Context, token string) (sessions []*sessionsearchv1pb.SessionSummary, nextToken string, err error)

// RunSearchTUI launches the full-screen interactive TUI for browsing session
// search results. It blocks until the user quits.
//
// nextToken is the cursor from the first BatchComplete response; pass "" when
// there are no more pages. fetcher is called when the user triggers "load
// more" and may be nil when nextToken is "". summaryGetter may be nil; the
// TUI degrades gracefully when summaries are not available.
func RunSearchTUI(
	ctx context.Context,
	sessions []*sessionsearchv1pb.SessionSummary,
	nextToken string,
	getter SummaryGetter,
	fetcher BatchFetcher,
) error {
	p := tea.NewProgram(newModel(ctx, sessions, nextToken, getter, fetcher), tea.WithAltScreen())
	_, err := p.Run()
	return trace.Wrap(err)
}
