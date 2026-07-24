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

package common

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/beams/replayfixture"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/session"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// BeamsCommand implements the "tctl beams" group of commands.
type BeamsCommand struct {
	config *servicecfg.Config

	// beamsExport implements "tctl beams export".
	beamsExport   *kingpin.CmdClause
	exportBeamID  string
	exportOutput  string
	exportFromUTC string
	exportToUTC   string

	// beamsRegenerate implements "tctl beams regenerate".
	beamsRegenerate  *kingpin.CmdClause
	regenerateBeamID string

	// stdout allows switching the output source in tests.
	stdout io.Writer
}

// Initialize plugs BeamsCommand into the CLI parser.
func (c *BeamsCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	if c.stdout == nil {
		c.stdout = os.Stdout
	}
	c.config = config

	beams := app.Command("beams", "Inspect and export beams.")
	export := beams.Command("export", "Export a beam's recorded activity to a replay fixture file for offline testing.")
	export.Arg("beam-id", "ID of the beam to export (the UUID from the beam session recording view).").Required().StringVar(&c.exportBeamID)
	export.Flag("output", "Output fixture file path.").Short('o').Default("beam-fixture.json").StringVar(&c.exportOutput)
	export.Flag("from-utc", fmt.Sprintf("Override the start of the time range searched for the beam's recordings. Format %s. Defaults to the beam's created time from its replay manifest.", defaults.TshTctlSessionListTimeFormat)).StringVar(&c.exportFromUTC)
	export.Flag("to-utc", fmt.Sprintf("Override the end of the time range. Format %s. Defaults to the beam's expiry from its replay manifest.", defaults.TshTctlSessionListTimeFormat)).StringVar(&c.exportToUTC)
	c.beamsExport = export

	regenerate := beams.Command("regenerate", "Re-run a beam's activity summary and replay artifact pipeline.")
	regenerate.Arg("beam-id", "ID of the beam to regenerate (the UUID from the beam session recording view).").Required().StringVar(&c.regenerateBeamID)
	c.beamsRegenerate = regenerate
}

// TryRun runs the matching "tctl beams" subcommand.
func (c *BeamsCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (bool, error) {
	var run func(context.Context, *authclient.Client) error
	switch cmd {
	case c.beamsExport.FullCommand():
		run = c.exportBeam
	case c.beamsRegenerate.FullCommand():
		run = c.regenerateBeam
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer closeFn(ctx)
	return true, trace.Wrap(run(ctx, client))
}

// regenerateBeam re-runs the beam's summary and replay artifact pipeline via
// the RegenerateBeamSummary RPC, which runs synchronously and returns once the
// pipeline completes.
func (c *BeamsCommand) regenerateBeam(ctx context.Context, tc *authclient.Client) error {
	req := beamsv1.RegenerateBeamSummaryRequest_builder{Name: c.regenerateBeamID}.Build()
	if _, err := tc.BeamServiceClient().RegenerateBeamSummary(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(c.stdout, "Regenerated summary and replay artifact for beam %q.\n", c.regenerateBeamID)
	return nil
}

func (c *BeamsCommand) exportBeam(ctx context.Context, tc *authclient.Client) error {
	// The replay manifest carries the beam's created/expires times (what the
	// web viewer opens) and persists for as long as the recording does, so use
	// it to bound the audit search. It is best-effort: fixtures can still be
	// exported from the raw recording with an explicit --from-utc/--to-utc.
	manifest, manifestErr := c.openManifest(ctx, tc)
	if manifestErr != nil {
		fmt.Fprintf(c.stdout, "Note: could not open beam replay manifest (%v); resolving the search window from flags/defaults.\n", manifestErr)
	}
	from, to := c.resolveWindow(manifest)

	beamEvents, err := c.collectBeamEvents(ctx, tc, from, to)
	if err != nil {
		return trace.Wrap(err, "collecting beam audit events")
	}
	chunkIDs := chunkIDsFromEvents(beamEvents)
	if len(chunkIDs) == 0 {
		return trace.NotFound("no app session chunks found for beam %q in %s..%s; widen the window with --from-utc/--to-utc",
			c.exportBeamID, from.Format(time.RFC3339), to.Format(time.RFC3339))
	}

	chunks := make(map[string][]apievents.AuditEvent, len(chunkIDs))
	chunkEventCount := 0
	for _, id := range chunkIDs {
		evs, err := drainSessionEvents(ctx, tc, id)
		if err != nil {
			return trace.Wrap(err, "reading chunk recording %q", id)
		}
		chunks[id] = evs
		chunkEventCount += len(evs)
	}

	fixture, err := replayfixture.New(jobParamsFromManifest(c.exportBeamID, manifest, from, to), beamEvents, chunks)
	if err != nil {
		return trace.Wrap(err, "building fixture")
	}
	if err := fixture.WriteFile(c.exportOutput); err != nil {
		return trace.Wrap(err, "writing fixture")
	}

	fmt.Fprintf(c.stdout, "Exported beam %q to %s (%d audit events, %d chunk recordings, %d chunk events).\n",
		c.exportBeamID, c.exportOutput, len(beamEvents), len(chunkIDs), chunkEventCount)
	return nil
}

// openManifest opens a replay session for the beam and returns its manifest,
// the same way the web viewer does (a framed open over the bidirectional
// Replay stream).
func (c *BeamsCommand) openManifest(ctx context.Context, tc *authclient.Client) (*beamsv1.ReplayManifest, error) {
	stream, err := tc.BeamReplayServiceClient().Replay(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() { _ = stream.CloseSend() }()

	open := beamsv1.OpenRequest_builder{BeamId: c.exportBeamID}.Build()
	req := beamsv1.ReplayRequest_builder{RequestId: "open", Open: open}.Build()
	if err := stream.Send(req); err != nil {
		return nil, trace.Wrap(err)
	}
	for {
		resp, err := stream.Recv()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if e := resp.GetError(); e != nil {
			return nil, trace.Errorf("%s", e.GetMessage())
		}
		if m := resp.GetManifest(); m != nil {
			return m, nil
		}
	}
}

// resolveWindow picks the audit search window: explicit flags win, then the
// manifest's created/expires (padded), else a bounded last-48h fallback.
func (c *BeamsCommand) resolveWindow(manifest *beamsv1.ReplayManifest) (time.Time, time.Time) {
	if c.exportFromUTC != "" || c.exportToUTC != "" {
		if from, to, err := defaults.SearchSessionRange(clockwork.NewRealClock(), c.exportFromUTC, c.exportToUTC, ""); err == nil {
			return from, to
		}
	}
	if manifest != nil {
		created := manifest.GetCreatedAt().AsTime()
		expires := manifest.GetExpiresAt().AsTime()
		if !created.IsZero() && !expires.IsZero() && expires.After(created) {
			// Pad both ends so events stamped slightly outside the reported
			// window are still captured.
			return created.Add(-time.Hour), expires.Add(time.Hour)
		}
	}
	now := time.Now().UTC()
	return now.Add(-48 * time.Hour), now.Add(time.Hour)
}

// collectBeamEvents pages through every audit event attributed to the beam.
func (c *BeamsCommand) collectBeamEvents(ctx context.Context, tc *authclient.Client, from, to time.Time) ([]apievents.AuditEvent, error) {
	var all []apievents.AuditEvent
	startKey := ""
	for {
		batch, next, err := tc.SearchEvents(ctx, events.SearchEventsRequest{
			From:     from,
			To:       to,
			BeamID:   c.exportBeamID,
			Limit:    apidefaults.DefaultChunkSize,
			Order:    types.EventOrderAscending,
			StartKey: startKey,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, ev := range batch {
			// SearchEvents filters server-side; re-check attribution to stay
			// robust to backends that ignore the filter.
			if bg, ok := ev.(interface{ GetBeamID() string }); !ok || bg.GetBeamID() != c.exportBeamID {
				continue
			}
			all = append(all, ev)
		}
		if next == "" {
			break
		}
		startKey = next
	}
	return all, nil
}

// chunkIDsFromEvents returns the app-session-chunk recording IDs referenced by
// the beam's events, in first-seen order, deduplicated.
func chunkIDsFromEvents(evs []apievents.AuditEvent) []string {
	seen := make(map[string]struct{})
	var ids []string
	for _, ev := range evs {
		chunk, ok := ev.(*apievents.AppSessionChunk)
		if !ok {
			continue
		}
		id := chunk.GetSessionChunkID()
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

// drainSessionEvents streams a chunk recording to completion. It mirrors the
// AuditLog contract: the event channel closes on success and the error channel
// receives only on failure.
func drainSessionEvents(ctx context.Context, tc *authclient.Client, chunkID string) ([]apievents.AuditEvent, error) {
	evCh, errCh := tc.StreamSessionEvents(ctx, session.ID(chunkID), 0)
	var out []apievents.AuditEvent
	for {
		select {
		case ev, ok := <-evCh:
			if !ok {
				return out, nil
			}
			out = append(out, ev)
		case err := <-errCh:
			if err != nil && !trace.IsEOF(err) {
				return nil, trace.Wrap(err)
			}
			return out, nil
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		}
	}
}

func jobParamsFromManifest(beamID string, m *beamsv1.ReplayManifest, from, to time.Time) replayfixture.JobParams {
	job := replayfixture.JobParams{BeamID: beamID, CreatedAt: from, ExpiresAt: to}
	if m == nil {
		return job
	}
	if id := m.GetBeamId(); id != "" {
		job.BeamID = id
	}
	job.Alias = m.GetAlias()
	job.Owner = m.GetOwner()
	job.AppName = m.GetPublishedAppName()
	if t := m.GetCreatedAt(); t != nil {
		job.CreatedAt = t.AsTime()
	}
	if t := m.GetExpiresAt(); t != nil {
		job.ExpiresAt = t.AsTime()
	}
	return job
}
