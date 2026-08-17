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
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// `tsh beams dev` turns a beam into the execution host for a local git
// checkout: the developer edits locally in any IDE while a foreground daemon
// keeps the two working trees converged (see beams_dev_sync.go) and rotates
// the beam before its 24h expiry. Agents (claude/codex) and builds run on the
// beam — never on the laptop — via `tsh beams dev run` and the generated
// shims.
type beamsDevCommand struct {
	attach  *beamsDevAttachCommand
	run     *beamsDevRunCommand
	shell   *beamsDevShellCommand
	status  *beamsDevStatusCommand
	detach  *beamsDevDetachCommand
	handoff *beamsDevHandoffCommand
}

func newBeamsDevCommand(parent *kingpin.CmdClause) *beamsDevCommand {
	dev := parent.Command("dev", "Use a beam as the execution host for a local git checkout (edit locally, run remotely).")
	return &beamsDevCommand{
		attach:  newBeamsDevAttachCommand(dev),
		run:     newBeamsDevRunCommand(dev),
		shell:   newBeamsDevShellCommand(dev),
		status:  newBeamsDevStatusCommand(dev),
		detach:  newBeamsDevDetachCommand(dev),
		handoff: newBeamsDevHandoffCommand(dev),
	}
}

// ---------------------------------------------------------------------------
// attach

type beamsDevAttachCommand struct {
	*kingpin.CmdClause
	dir         string
	interval    time.Duration
	autoHandoff bool
	noShims     bool
}

func newBeamsDevAttachCommand(parent *kingpin.CmdClause) *beamsDevAttachCommand {
	cmd := &beamsDevAttachCommand{
		CmdClause: parent.Command("attach", "Attach a local git checkout to a beam and start the sync loop.").Default(),
	}
	cmd.Arg("dir", "Local git checkout to attach (defaults to the current directory).").Default(".").StringVar(&cmd.dir)
	cmd.Flag("interval", "Sync cycle interval.").Default("3s").DurationVar(&cmd.interval)
	cmd.Flag("auto-handoff", "Rotate to a successor beam before the current one expires.").Default("true").BoolVar(&cmd.autoHandoff)
	cmd.Flag("no-shims", "Do not generate .beams/bin command shims in the checkout.").BoolVar(&cmd.noShims)
	return cmd
}

// warmHandoffLead is how long before beam expiry the client performs a warm
// handoff. The beam-side self-handoff cron fires at 90 minutes, so a healthy
// attached client always beats it.
const warmHandoffLead = 100 * time.Minute

func (c *beamsDevAttachCommand) run(cf *CLIConf) error {
	ctx := cf.Context

	root, err := beamsDevResolveRepoRoot(ctx, c.dir)
	if err != nil {
		return trace.Wrap(err)
	}

	ws, err := loadDevWorkspace(root)
	if err != nil {
		return trace.Wrap(err)
	}
	freshWorkspace := ws == nil
	if freshWorkspace {
		branch, err := runLocalGit(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return trace.Wrap(err)
		}
		originURL, _ := runLocalGit(ctx, root, "config", "--get", "remote.origin.url")
		repoName := filepath.Base(root)
		ws = &devWorkspace{
			ID:        beamsDevWorkspaceID(root),
			LocalDir:  root,
			RepoName:  repoName,
			RemoteDir: beamsDevRemoteHome + "/" + repoName,
			Branch:    branch,
			OriginURL: originURL,
			CreatedAt: time.Now().UTC(),
		}
		if _, err := os.Stat(filepath.Join(root, ".beams", "setup.sh")); err == nil {
			ws.SetupScript = ".beams/setup.sh"
		}
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	tc.AllowHeadless = true

	beam, freshBeam, err := beamsDevResolveBeam(cf, tc, ws)
	if err != nil {
		return trace.Wrap(err)
	}
	ws.BeamUUID = beam.GetMetadata().GetName()
	ws.BeamAlias = beam.GetStatus().GetAlias()
	ws.BeamExpires = beam.GetSpec().GetExpires().AsTime()
	if err := saveDevWorkspace(ws); err != nil {
		return trace.Wrap(err)
	}

	runner, err := newBeamRunner(cf, beam)
	if err != nil {
		return trace.Wrap(err)
	}
	engine, err := newDevSyncEngine(cf, ws, runner)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(cf.Stdout(), "Workspace %s ⇄ beam %q (expires %s)\n",
		ws.RepoName, ws.BeamAlias, ws.BeamExpires.Local().Format(time.RFC822))

	// An adopted or previously attached beam may still lack the payload if an
	// earlier attach was interrupted mid-provision — trust the on-beam marker,
	// not just whether we created the beam this run.
	if !freshBeam {
		freshBeam = readBeamsDevWorkspaceMarker(ctx, runner) != ws.ID
	}

	if err := engine.seed(ctx); err != nil {
		return trace.Wrap(err)
	}
	// The payload is refreshed on EVERY attach (idempotent; the installer
	// guards against duplicate pollers) so script fixes reach existing beams.
	// The setup script still runs only on fresh beams.
	if err := installBeamsDevPayload(ctx, runner, ws); err != nil {
		return trace.Wrap(err)
	}
	if freshBeam {
		if err := beamsDevRunSetup(ctx, cf, runner, ws); err != nil {
			return trace.Wrap(err)
		}
	}
	if !c.noShims {
		if err := beamsDevWriteShims(ws); err != nil {
			fmt.Fprintf(cf.Stdout(), "! could not write command shims: %v\n", err)
		}
	}

	fmt.Fprintf(cf.Stdout(), "Synced. Edit locally; run `claude` (via .beams/bin shim) or `tsh beams dev run -- <cmd>` to execute on the beam. Ctrl+C detaches (workspace stays attached).\n")

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	refresh := time.NewTicker(5 * time.Minute)
	defer refresh.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(cf.Stdout(), "\nSync stopped. Reattach anytime with: tsh beams dev %s\n", ws.LocalDir)
			return nil
		case <-ticker.C:
			engine.syncCycle(ctx)
		case <-refresh.C:
			current, err := beamsDevGetBeam(cf, tc, ws.BeamUUID)
			switch {
			case err != nil && trace.IsNotFound(err):
				fmt.Fprintf(cf.Stdout(), "! beam %q is gone; recovering...\n", ws.BeamAlias)
				recovered, fresh, err := beamsDevResolveBeam(cf, tc, ws)
				if err != nil {
					return trace.Wrap(err)
				}
				beam = recovered
				runner.setBeam(beam)
				ws.BeamUUID = beam.GetMetadata().GetName()
				ws.BeamAlias = beam.GetStatus().GetAlias()
				ws.BeamExpires = beam.GetSpec().GetExpires().AsTime()
				_ = saveDevWorkspace(ws)
				if !fresh {
					fresh = readBeamsDevWorkspaceMarker(ctx, runner) != ws.ID
				}
				if err := engine.seed(ctx); err != nil {
					return trace.Wrap(err)
				}
				if fresh {
					if err := installBeamsDevPayload(ctx, runner, ws); err != nil {
						return trace.Wrap(err)
					}
					if err := beamsDevRunSetup(ctx, cf, runner, ws); err != nil {
						return trace.Wrap(err)
					}
				}
			case err != nil:
				fmt.Fprintf(cf.Stdout(), "! refreshing beam state: %v\n", err)
			default:
				beam = current
				runner.setBeam(beam)
				ws.BeamExpires = beam.GetSpec().GetExpires().AsTime()
				if beamsDevAdoptSuccessor(cf, tc, runner, ws) {
					// The beam handed itself off behind our back (e.g. the
					// warm handoff lost a race against the beam-side cron);
					// the runner now points at the successor.
					beam = runner.beam
				} else if c.autoHandoff && time.Until(ws.BeamExpires) < warmHandoffLead {
					if err := beamsDevWarmHandoff(cf, tc, runner, ws, false); err != nil {
						fmt.Fprintf(cf.Stdout(), "! warm handoff failed (beam-side self-handoff remains as backstop): %v\n", err)
					} else {
						beam = runner.beam
					}
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// run / shell

type beamsDevRunCommand struct {
	*kingpin.CmdClause
	command []string
}

func newBeamsDevRunCommand(parent *kingpin.CmdClause) *beamsDevRunCommand {
	cmd := &beamsDevRunCommand{
		CmdClause: parent.Command("run", "Run a command on the workspace's beam, in the directory mapped from the current one."),
	}
	cmd.Arg("command", "Command to execute on the beam.").Required().StringsVar(&cmd.command)
	return cmd
}

func (c *beamsDevRunCommand) run(cf *CLIConf) error {
	quoted := make([]string, 0, len(c.command))
	for _, arg := range c.command {
		quoted = append(quoted, beamsDevShellQuote(arg))
	}
	return trace.Wrap(beamsDevRunMapped(cf, "exec "+strings.Join(quoted, " ")))
}

type beamsDevShellCommand struct {
	*kingpin.CmdClause
}

func newBeamsDevShellCommand(parent *kingpin.CmdClause) *beamsDevShellCommand {
	return &beamsDevShellCommand{
		CmdClause: parent.Command("shell", "Open an interactive shell on the workspace's beam, in the directory mapped from the current one."),
	}
}

func (c *beamsDevShellCommand) run(cf *CLIConf) error {
	return trace.Wrap(beamsDevRunMapped(cf, "exec bash -l"))
}

// beamsDevRunMapped executes remoteCmd on the workspace beam with an
// interactive TTY, in the beam directory corresponding to the local cwd.
func beamsDevRunMapped(cf *CLIConf, remoteCmd string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return trace.Wrap(err)
	}
	root, err := beamsDevResolveRepoRoot(cf.Context, cwd)
	if err != nil {
		return trace.Wrap(err)
	}
	ws, err := loadDevWorkspace(root)
	if err != nil {
		return trace.Wrap(err)
	}
	if ws == nil {
		return trace.NotFound("no beams-dev workspace for %s; run `tsh beams dev` there first", root)
	}

	rel, err := filepath.Rel(root, cwd)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = "."
	}
	remoteDir := ws.RemoteDir
	if rel != "." {
		remoteDir = ws.RemoteDir + "/" + filepath.ToSlash(rel)
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	tc.AllowHeadless = true
	tc.InteractiveCommand = true

	beam, err := beamsDevGetBeam(cf, tc, ws.BeamUUID)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("beam %q is gone; run `tsh beams dev %s` to recover the workspace first", ws.BeamAlias, ws.LocalDir)
		}
		return trace.Wrap(err)
	}

	full := fmt.Sprintf("cd %s && %s", beamsDevShellQuote(remoteDir), remoteCmd)
	return trace.Wrap(sshBeam(cf, tc, beam, []string{full}))
}

// ---------------------------------------------------------------------------
// status / detach

type beamsDevStatusCommand struct {
	*kingpin.CmdClause
	dir string
}

func newBeamsDevStatusCommand(parent *kingpin.CmdClause) *beamsDevStatusCommand {
	cmd := &beamsDevStatusCommand{
		CmdClause: parent.Command("status", "Show the beams-dev workspace attached to a local checkout."),
	}
	cmd.Arg("dir", "Local checkout (defaults to the current directory).").Default(".").StringVar(&cmd.dir)
	return cmd
}

func (c *beamsDevStatusCommand) run(cf *CLIConf) error {
	root, err := beamsDevResolveRepoRoot(cf.Context, c.dir)
	if err != nil {
		return trace.Wrap(err)
	}
	ws, err := loadDevWorkspace(root)
	if err != nil {
		return trace.Wrap(err)
	}
	if ws == nil {
		fmt.Fprintf(cf.Stdout(), "No beams-dev workspace for %s.\n", root)
		return nil
	}

	fmt.Fprintf(cf.Stdout(), "Workspace: %s (%s)\n", ws.RepoName, ws.ID)
	fmt.Fprintf(cf.Stdout(), "Local:     %s (branch %s)\n", ws.LocalDir, ws.Branch)
	fmt.Fprintf(cf.Stdout(), "Beam:      %s (%s), repo at %s\n", ws.BeamAlias, ws.BeamUUID, ws.RemoteDir)

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	beam, err := beamsDevGetBeam(cf, tc, ws.BeamUUID)
	switch {
	case trace.IsNotFound(err):
		fmt.Fprintf(cf.Stdout(), "State:     beam is GONE — reattach with `tsh beams dev %s` to recover (successor or fresh seed)\n", ws.LocalDir)
	case err != nil:
		return trace.Wrap(err)
	default:
		fmt.Fprintf(cf.Stdout(), "Expires:   %s (%s)\n",
			beam.GetSpec().GetExpires().AsTime().Local().Format(time.RFC822),
			time.Until(beam.GetSpec().GetExpires().AsTime()).Round(time.Minute))
	}
	return nil
}

type beamsDevDetachCommand struct {
	*kingpin.CmdClause
	dir        string
	deleteBeam bool
}

func newBeamsDevDetachCommand(parent *kingpin.CmdClause) *beamsDevDetachCommand {
	cmd := &beamsDevDetachCommand{
		CmdClause: parent.Command("detach", "Forget the workspace attachment for a local checkout."),
	}
	cmd.Arg("dir", "Local checkout (defaults to the current directory).").Default(".").StringVar(&cmd.dir)
	cmd.Flag("rm-beam", "Also delete the attached beam.").BoolVar(&cmd.deleteBeam)
	return cmd
}

func (c *beamsDevDetachCommand) run(cf *CLIConf) error {
	ctx := cf.Context
	root, err := beamsDevResolveRepoRoot(ctx, c.dir)
	if err != nil {
		return trace.Wrap(err)
	}
	ws, err := loadDevWorkspace(root)
	if err != nil {
		return trace.Wrap(err)
	}
	if ws == nil {
		fmt.Fprintf(cf.Stdout(), "No beams-dev workspace for %s.\n", root)
		return nil
	}

	if c.deleteBeam && ws.BeamUUID != "" {
		tc, err := makeClient(cf)
		if err != nil {
			return trace.Wrap(err)
		}
		err = beamsDevWithRootClient(cf, tc, func(rootClient authclient.ClientI) error {
			_, err := rootClient.BeamServiceClient().DeleteBeam(ctx, beamsv1.DeleteBeamRequest_builder{
				Name: ws.BeamUUID,
			}.Build())
			return trace.Wrap(err)
		})
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		fmt.Fprintf(cf.Stdout(), "Deleted beam %q.\n", ws.BeamAlias)
	}

	_ = os.RemoveAll(filepath.Join(ws.LocalDir, ".beams", "bin"))
	if err := deleteDevWorkspace(ws.ID); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(cf.Stdout(), "Workspace %s detached.\n", ws.RepoName)
	return nil
}

// ---------------------------------------------------------------------------
// Beam resolution, handoff, setup, shims

func beamsDevResolveRepoRoot(ctx context.Context, dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	root, err := runLocalGit(ctx, abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", trace.BadParameter("%s is not inside a git checkout: %v", abs, err)
	}
	return root, nil
}

func beamsDevWithRootClient(cf *CLIConf, tc *client.TeleportClient, fn func(authclient.ClientI) error) error {
	return trace.Wrap(client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		rootClient, err := clusterClient.ConnectToRootCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rootClient.Close()

		return trace.Wrap(fn(rootClient))
	}))
}

func beamsDevGetBeam(cf *CLIConf, tc *client.TeleportClient, ref string) (*beamsv1.Beam, error) {
	var beam *beamsv1.Beam
	err := beamsDevWithRootClient(cf, tc, func(rootClient authclient.ClientI) error {
		var err error
		beam, err = getBeam(cf.Context, rootClient, ref)
		return trace.Wrap(err)
	})
	return beam, trace.Wrap(err)
}

func beamsDevCreateBeam(cf *CLIConf, tc *client.TeleportClient) (*beamsv1.Beam, error) {
	// Best-effort proxy region discovery, matching `tsh beams add`.
	proxyRegion := ""
	if resp, err := webclient.Find(&webclient.Config{
		Context:      cf.Context,
		ProxyAddr:    tc.WebProxyAddr,
		Insecure:     tc.InsecureSkipVerify,
		ExtraHeaders: tc.ExtraProxyHeaders,
	}); err == nil {
		proxyRegion = resp.Proxy.GroupID
	}

	var beam *beamsv1.Beam
	err := beamsDevWithRootClient(cf, tc, func(rootClient authclient.ClientI) error {
		rsp, err := rootClient.BeamServiceClient().CreateBeam(cf.Context, beamsv1.CreateBeamRequest_builder{
			Egress:      beamsv1.EgressMode_EGRESS_MODE_UNRESTRICTED,
			ProxyRegion: proxyRegion,
		}.Build())
		if err != nil {
			return trace.Wrap(err)
		}
		beam = rsp.GetBeam()
		return nil
	})
	return beam, trace.Wrap(err)
}

func beamsDevListOwnBeams(cf *CLIConf, tc *client.TeleportClient) ([]*beamsv1.Beam, error) {
	var beams []*beamsv1.Beam
	err := beamsDevWithRootClient(cf, tc, func(rootClient authclient.ClientI) error {
		user, err := rootClient.GetCurrentUser(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		filters := &beamsv1.ListBeamsRequest_Filters{}
		filters.SetUsers([]string{user.GetName()})

		beams, err = stream.Collect(
			clientutils.Resources(cf.Context, func(ctx context.Context, pageSize int, pageToken string) ([]*beamsv1.Beam, string, error) {
				rsp, err := rootClient.BeamServiceClient().ListBeams(ctx, beamsv1.ListBeamsRequest_builder{
					PageSize:  int32(pageSize),
					PageToken: pageToken,
					Filters:   filters,
				}.Build())
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				return rsp.GetBeams(), rsp.GetNextPageToken(), nil
			}))
		return trace.Wrap(err)
	})
	return beams, trace.Wrap(err)
}

// beamsDevResolveBeam finds the beam for a workspace: the recorded one if
// still alive, else a successor advertised by a marker on one of the user's
// beams (covers laptop-off self-handoffs), else a freshly created beam.
// The second return value reports whether the beam needs payload+setup.
func beamsDevResolveBeam(cf *CLIConf, tc *client.TeleportClient, ws *devWorkspace) (*beamsv1.Beam, bool, error) {
	if ws.BeamUUID != "" {
		beam, err := beamsDevGetBeam(cf, tc, ws.BeamUUID)
		if err == nil {
			// The beam may have already handed itself off (the T-90min
			// self-handoff fires with no laptop attached) while still being
			// alive until expiry. A drained predecessor must not win over
			// its successor.
			if runner, rerr := newBeamRunner(cf, beam); rerr == nil {
				if beamsDevAdoptSuccessor(cf, tc, runner, ws) {
					return runner.beam, false, nil
				}
			}
			return beam, false, nil
		}
		if !trace.IsNotFound(err) {
			return nil, false, trace.Wrap(err)
		}
		fmt.Fprintf(cf.Stdout(), "Beam %q has expired; looking for a successor...\n", ws.BeamAlias)

		beams, err := beamsDevListOwnBeams(cf, tc)
		if err != nil {
			return nil, false, trace.Wrap(err)
		}
		// One pass classifies every beam belonging to this workspace: a marker
		// means fully seeded (a completed self-handoff), a claim means an
		// interrupted handoff got as far as creating it. The best match is
		// adopted (seeded beats claimed, then youngest); the rest are strays
		// and deleted, so failed handoffs can never accumulate beams.
		type wsMatch struct {
			beam   *beamsv1.Beam
			seeded bool
		}
		var matches []wsMatch
		for _, candidate := range beams {
			if candidate.GetMetadata().GetName() == ws.BeamUUID {
				continue
			}
			runner, err := newBeamRunner(cf, candidate)
			if err != nil {
				continue
			}
			marker, claim := readBeamsDevBeamState(cf.Context, runner)
			if marker == ws.ID || claim == ws.ID {
				matches = append(matches, wsMatch{beam: candidate, seeded: marker == ws.ID})
			}
		}
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].seeded != matches[j].seeded {
				return matches[i].seeded
			}
			return matches[i].beam.GetSpec().GetExpires().AsTime().After(matches[j].beam.GetSpec().GetExpires().AsTime())
		})
		if len(matches) > 0 {
			for _, stray := range matches[1:] {
				alias := stray.beam.GetStatus().GetAlias()
				if err := beamsDevDeleteBeam(cf, tc, stray.beam.GetMetadata().GetName()); err != nil {
					fmt.Fprintf(cf.Stdout(), "! could not delete stray workspace beam %q (it will expire on its own): %v\n", alias, err)
				} else {
					fmt.Fprintf(cf.Stdout(), "Deleted stray beam %q left behind by an interrupted handoff.\n", alias)
				}
			}
			chosen := matches[0]
			if chosen.seeded {
				fmt.Fprintf(cf.Stdout(), "Adopted successor beam %q (self-handoff carried the workspace over).\n",
					chosen.beam.GetStatus().GetAlias())
				return chosen.beam, false, nil
			}
			fmt.Fprintf(cf.Stdout(), "Reusing beam %q: an interrupted handoff claimed it for this workspace but never finished seeding (environment setup will re-run).\n",
				chosen.beam.GetStatus().GetAlias())
			return chosen.beam, true, nil
		}
		fmt.Fprintf(cf.Stdout(), "No successor found; creating a fresh beam (environment setup will re-run).\n")
	}

	fmt.Fprintf(cf.Stdout(), "Creating beam...\n")
	beam, err := beamsDevCreateBeam(cf, tc)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	return beam, true, nil
}

// beamsDevHandoffCommand triggers the beam-to-beam handoff on demand — the
// same path the TTL watcher takes at T-100min, useful for demos and for
// rotating ahead of planned downtime.
type beamsDevHandoffCommand struct {
	*kingpin.CmdClause
	dir string
}

func newBeamsDevHandoffCommand(parent *kingpin.CmdClause) *beamsDevHandoffCommand {
	cmd := &beamsDevHandoffCommand{
		CmdClause: parent.Command("handoff", "Rotate the workspace onto a fresh beam now, carrying the environment and agent context across."),
	}
	cmd.Arg("dir", "Local checkout (defaults to the current directory).").Default(".").StringVar(&cmd.dir)
	return cmd
}

func (c *beamsDevHandoffCommand) run(cf *CLIConf) error {
	root, err := beamsDevResolveRepoRoot(cf.Context, c.dir)
	if err != nil {
		return trace.Wrap(err)
	}
	ws, err := loadDevWorkspace(root)
	if err != nil {
		return trace.Wrap(err)
	}
	if ws == nil {
		return trace.NotFound("no beams-dev workspace for %s; run `tsh beams dev` there first", root)
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	tc.AllowHeadless = true

	beam, err := beamsDevGetBeam(cf, tc, ws.BeamUUID)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("beam %q is already gone; run `tsh beams dev %s` to recover instead", ws.BeamAlias, ws.LocalDir)
		}
		return trace.Wrap(err)
	}
	ws.BeamExpires = beam.GetSpec().GetExpires().AsTime()

	runner, err := newBeamRunner(cf, beam)
	if err != nil {
		return trace.Wrap(err)
	}
	// Manual handoff is an explicit user decision — don't defer on active
	// agent sessions the way the automatic TTL rotation does.
	return trace.Wrap(beamsDevWarmHandoff(cf, tc, runner, ws, true))
}

// beamsDevDeleteBeam deletes a beam by UUID (drained predecessors and strays
// from interrupted handoffs — deleting promptly frees the concurrent quota).
func beamsDevDeleteBeam(cf *CLIConf, tc *client.TeleportClient, uuid string) error {
	return trace.Wrap(beamsDevWithRootClient(cf, tc, func(rootClient authclient.ClientI) error {
		_, err := rootClient.BeamServiceClient().DeleteBeam(cf.Context, beamsv1.DeleteBeamRequest_builder{
			Name: uuid,
		}.Build())
		return trace.Wrap(err)
	}))
}

// beamsDevAdoptSuccessor checks a live beam for a self-handoff pointer and, if
// one resolves, repoints the workspace (and runner) at the successor and
// deletes the drained predecessor. Returns true when a successor was adopted.
func beamsDevAdoptSuccessor(cf *CLIConf, tc *client.TeleportClient, runner *beamRunner, ws *devWorkspace) bool {
	uuid, alias, ok := readBeamsDevSuccessor(cf.Context, runner)
	if !ok {
		return false
	}
	successor, err := beamsDevGetBeam(cf, tc, uuid)
	if err != nil {
		fmt.Fprintf(cf.Stdout(), "! beam %q points at successor %q but it is unreachable (%v); staying on the current beam\n",
			ws.BeamAlias, alias, err)
		return false
	}

	oldUUID, oldAlias := ws.BeamUUID, ws.BeamAlias
	runner.setBeam(successor)
	ws.BeamUUID = successor.GetMetadata().GetName()
	ws.BeamAlias = successor.GetStatus().GetAlias()
	ws.BeamExpires = successor.GetSpec().GetExpires().AsTime()
	if err := saveDevWorkspace(ws); err != nil {
		fmt.Fprintf(cf.Stdout(), "! could not persist workspace record: %v\n", err)
	}

	if err := beamsDevDeleteBeam(cf, tc, oldUUID); err != nil && !trace.IsNotFound(err) {
		fmt.Fprintf(cf.Stdout(), "! could not delete predecessor beam %q (it will expire on its own): %v\n", oldAlias, err)
	}

	fmt.Fprintf(cf.Stdout(), "Beam %q had already handed off to %q; adopted the successor and deleted the old beam.\n",
		oldAlias, ws.BeamAlias)
	return true
}

// beamsDevWarmHandoff rotates the workspace onto a fresh beam while the old
// one is still alive: everything under /home/beams that matters (repo with
// caches, Claude sessions, payload) moves beam-to-beam through the old beam's
// own delegated identity, so nothing round-trips through the laptop.
func beamsDevWarmHandoff(cf *CLIConf, tc *client.TeleportClient, runner *beamRunner, ws *devWorkspace, force bool) error {
	ctx := cf.Context

	// A live process cannot move between VMs: rotating now would tar the
	// transcript mid-turn and then delete the beam under the running agent.
	// With runway left, defer and let the next refresh retry; inside the
	// force window, rotate anyway — losing the last turn beats losing the
	// whole workspace to expiry.
	const handoffForceWindow = 40 * time.Minute
	if !force && time.Until(ws.BeamExpires) > handoffForceWindow {
		out, err := runner.output(ctx, "pgrep -f 'claude|codex' 2>/dev/null | head -3; true")
		if err == nil && strings.TrimSpace(out) != "" {
			fmt.Fprintf(cf.Stdout(), "Deferring handoff: an agent session is active on beam %q; retrying at the next refresh (rotation is forced once <%s of life remains).\n",
				ws.BeamAlias, handoffForceWindow)
			return nil
		}
	}

	fmt.Fprintf(cf.Stdout(), "Beam %q expires in %s; rotating to a successor...\n",
		ws.BeamAlias, time.Until(ws.BeamExpires).Round(time.Minute))

	// Reuse the beam-side handoff script: it creates the successor, copies
	// the workspace across, and bootstraps the payload. Forcing EXPIRES_AT to
	// zero makes its threshold check fire immediately; `keepalive` stops the
	// script from deleting the old beam itself — we verify the successor
	// first and delete the predecessor below.
	_, err := runner.output(ctx, fmt.Sprintf(
		"rm -f %s/handoff.done && sed -i 's/^EXPIRES_AT=.*/EXPIRES_AT=0/' %s/meta.env && %s/handoff.sh keepalive; sed -i 's/^EXPIRES_AT=.*/EXPIRES_AT=%d/' %s/meta.env",
		beamsDevRemoteMetaDir, beamsDevRemoteMetaDir, beamsDevRemoteMetaDir, ws.BeamExpires.Unix(), beamsDevRemoteMetaDir))
	if err != nil {
		return trace.Wrap(err)
	}

	uuid, alias, ok := readBeamsDevSuccessor(ctx, runner)
	if !ok {
		out, _ := runner.output(ctx, fmt.Sprintf("tail -5 %s/handoff.log 2>/dev/null || true", beamsDevRemoteMetaDir))
		return trace.Errorf("handoff script left no successor pointer; recent log: %s", strings.TrimSpace(out))
	}

	successor, err := beamsDevGetBeam(cf, tc, uuid)
	if err != nil {
		return trace.Wrap(err, "successor beam %q not reachable", alias)
	}

	oldUUID, oldAlias := ws.BeamUUID, ws.BeamAlias
	runner.setBeam(successor)
	ws.BeamUUID = successor.GetMetadata().GetName()
	ws.BeamAlias = successor.GetStatus().GetAlias()
	ws.BeamExpires = successor.GetSpec().GetExpires().AsTime()
	if err := saveDevWorkspace(ws); err != nil {
		return trace.Wrap(err)
	}
	if err := refreshBeamsDevExpiry(ctx, runner, ws.BeamExpires); err != nil {
		fmt.Fprintf(cf.Stdout(), "! could not refresh successor expiry metadata: %v\n", err)
	}

	// The old beam would expire on its own; deleting it promptly frees the
	// concurrent-beams quota.
	if err := beamsDevDeleteBeam(cf, tc, oldUUID); err != nil {
		fmt.Fprintf(cf.Stdout(), "! could not delete old beam %q (it will expire on its own): %v\n", oldAlias, err)
	}

	fmt.Fprintf(cf.Stdout(), "Rotated %q -> %q (expires %s). Environment and Claude sessions carried over.\n",
		oldAlias, ws.BeamAlias, ws.BeamExpires.Local().Format(time.RFC822))
	return nil
}

// beamsDevRunSetup streams the repo's provisioning script (.beams/setup.sh)
// on a fresh beam.
func beamsDevRunSetup(ctx context.Context, cf *CLIConf, runner *beamRunner, ws *devWorkspace) error {
	if ws.SetupScript == "" {
		return nil
	}
	fmt.Fprintf(cf.Stdout(), "Running %s on beam %q...\n", ws.SetupScript, ws.BeamAlias)
	err := runner.run(ctx, fmt.Sprintf("cd %s && bash %s",
		beamsDevShellQuote(ws.RemoteDir), beamsDevShellQuote(ws.SetupScript)))
	if err != nil {
		return trace.Wrap(err, "setup script failed on beam %q", ws.BeamAlias)
	}
	return nil
}

// beamsDevShimCommands are the commands that must never run locally: their
// shims transparently re-execute them on the workspace's beam.
var beamsDevShimCommands = []string{"claude", "codex"}

const beamsDevShimTemplate = `#!/bin/sh
# beams-dev shim: this command runs on the workspace's beam, not locally.
exec %s beams dev run -- %s "$@"
`

// beamsDevWriteShims generates .beams/bin/<cmd> shims and hides .beams/bin
// from git via .git/info/exclude (never by editing tracked files).
func beamsDevWriteShims(ws *devWorkspace) error {
	binDir := filepath.Join(ws.LocalDir, ".beams", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return trace.Wrap(err)
	}
	// Embed the absolute path of the tsh that generated the shim: the stock
	// tsh on PATH may predate `beams dev`, and the shim must keep working in
	// shells whose PATH doesn't include a dev build.
	tshPath, err := os.Executable()
	if err != nil || tshPath == "" {
		tshPath = "tsh"
	}
	for _, name := range beamsDevShimCommands {
		shim := fmt.Sprintf(beamsDevShimTemplate, beamsDevShellQuote(tshPath), name)
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(shim), 0o755); err != nil {
			return trace.Wrap(err)
		}
	}

	excludePath := filepath.Join(ws.LocalDir, ".git", "info", "exclude")
	existing, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		// Worktrees have a .git file, not a directory; resolve via git.
		if common, gitErr := runLocalGit(context.Background(), ws.LocalDir, "rev-parse", "--git-common-dir"); gitErr == nil {
			if !filepath.IsAbs(common) {
				common = filepath.Join(ws.LocalDir, common)
			}
			excludePath = filepath.Join(common, "info", "exclude")
			existing, err = os.ReadFile(excludePath)
			if err != nil && !os.IsNotExist(err) {
				return trace.Wrap(err)
			}
		}
	}
	if !strings.Contains(string(existing), ".beams/bin/") {
		if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
			return trace.Wrap(err)
		}
		f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return trace.Wrap(err)
		}
		defer f.Close()
		if _, err := f.WriteString(".beams/bin/\n"); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
