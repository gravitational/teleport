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
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/spinner"
	toolcommon "github.com/gravitational/teleport/tool/common"
)

const (
	phaseSnapshotModeUnprotected = "unprotected"
	phaseSnapshotModeProtected   = "protected"
	phaseSnapshotModeAll         = "all"
)

type phasePublishCommand struct {
	*kingpin.CmdClause
	name   string
	from   string
	format string
	final  bool
	force  bool
	mode   string
}

type phasePublication struct {
	req        *beamsv1.PublishPhaseRequest
	generation string
	beamID     string
}

func newPhasePublishCommand(parent *kingpin.CmdClause) *phasePublishCommand {
	c := &phasePublishCommand{CmdClause: parent.Command("snapshot", "Save a snapshot to its existing Phase.")}
	c.Alias(phaseSnapshotHelp)
	c.Arg("name", "Existing Phase to save the snapshot to. Defaults to the local build Beam's Phase.").StringVar(&c.name)
	c.Flag("from", "Snapshot source: this Phase's name or parent@sha256:digest. Omit to capture the local build Beam.").StringVar(&c.from)
	c.Flag("final", "Save the local snapshot and end this build. Changes after the snapshot are discarded.").BoolVar(&c.final)
	c.Flag("force",
		"Exclude changes outside /home and home paths reserved for rebuilds. "+
			"Files in the running Beam are unchanged.",
	).BoolVar(&c.force)
	c.Flag("mode", "Changes to save in a fresh local snapshot.").
		Hidden().
		Default(phaseSnapshotModeUnprotected).
		EnumVar(&c.mode, phaseSnapshotModeUnprotected, phaseSnapshotModeProtected, phaseSnapshotModeAll)
	c.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON, teleport.YAML)
	return c
}

func (c *phasePublishCommand) snapshotArgs() []string {
	args := []string{"--mode=" + c.mode}
	if c.force {
		args = append(args, "--force")
	}
	return args
}

func (c *phasePublishCommand) target(env phaseEnvironment) (string, error) {
	if c.name != "" {
		return c.name, nil
	}
	name, err := env.PhaseName()
	if err != nil {
		return "", trace.BadParameter("specify a Phase name outside a build Beam")
	}
	return name, nil
}

func (c *phasePublishCommand) snapshotReference(
	ctx context.Context,
	get func(context.Context, string) (*beamsv1.Phase, error),
	env phaseEnvironment,
) (string, string, error) {
	if c.from != "" {
		snapshot, err := resolvePhaseSnapshot(ctx, c.from, get)
		return snapshot.parent, snapshot.digest, trace.Wrap(err)
	}
	ref, err := env.CreateSnapshot(ctx)
	if err != nil {
		return "", "", trace.Wrap(err, "running local beam-snapshot. Outside a build Beam, supply --from=Phase or --from=parent@sha256:digest")
	}
	return services.ParsePhaseSnapshot(ref)
}

func (c *phasePublishCommand) prepareRequest(
	ctx context.Context,
	name string,
	get func(context.Context, string) (*beamsv1.Phase, error),
	env phaseEnvironment,
) (*phasePublication, error) {
	// Capture the saved generation before uploading. Cleanup can change the
	// resource revision without changing this generation.
	current, err := get(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if c.final {
		if c.from != "" {
			return nil, trace.BadParameter("--final requires a fresh local snapshot and cannot be combined with --from")
		}
		if err := validatePhaseFinalBuild(current, env.BeamID()); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if c.from != "" &&
		!strings.Contains(c.from, "@") &&
		c.from != name {
		return nil, trace.BadParameter("cross-Phase snapshotting is not allowed. Use add --fork")
	}
	parent, digest, err := c.snapshotReference(ctx, get, env)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	publication := newPhasePublication(current, parent, digest)
	if c.final {
		publication.beamID = env.BeamID()
	}
	return publication, nil
}

func newPhasePublication(current *beamsv1.Phase, parent, digest string) *phasePublication {
	return &phasePublication{
		req: beamsv1.PublishPhaseRequest_builder{
			Name:             current.GetMetadata().GetName(),
			Parent:           parent,
			ImageDigest:      digest,
			ExpectedRevision: current.GetMetadata().GetRevision(),
		}.Build(),
		generation: current.GetStatus().GetPublished().GetGeneration(),
	}
}

func (publication *phasePublication) publish(ctx context.Context, api beamsv1.PhaseServiceClient) (*beamsv1.Phase, error) {
	for attempt := 1; ; attempt++ {
		resp, err := api.PublishPhase(ctx, publication.req)
		if err == nil {
			return resp.GetPhase(), nil
		}
		if !trace.IsCompareFailed(err) || attempt == 3 {
			return nil, phasePublicationError(err)
		}
		current, err := api.GetPhase(ctx, beamsv1.GetPhaseRequest_builder{
			Name: publication.req.GetName(),
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		phase := current.GetPhase()
		if phase.GetStatus().GetPublished().GetGeneration() != publication.generation {
			return nil, trace.CompareFailed("Phase snapshot changed since this command started. Run the command again to retry")
		}
		if publication.beamID != "" {
			if err := validatePhaseFinalBuild(phase, publication.beamID); err != nil {
				return nil, trace.Wrap(err)
			}
		}
		publication.req.SetExpectedRevision(phase.GetMetadata().GetRevision())
	}
}

func (c *phasePublishCommand) run(cf *CLIConf) error {
	var diagnostics bytes.Buffer
	beam := localBuildBeam(&diagnostics)
	beam.snapshotArgs = c.snapshotArgs()
	name, err := c.target(beam)
	if err != nil {
		return trace.Wrap(err)
	}
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	var publication *phasePublication
	var published *beamsv1.Phase
	// Keep the uploaded snapshot and original generation across relogin retries.
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
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
		if utils.IsTerminal(cf.Stderr()) {
			progress := spinner.New(cf.Stderr(), "Saving snapshot...")
			defer progress.Stop()
		}
		api := rootClient.PhaseServiceClient()
		if publication == nil {
			publication, err = c.prepareRequest(cf.Context, name, func(ctx context.Context, name string) (*beamsv1.Phase, error) {
				resp, err := api.GetPhase(ctx, beamsv1.GetPhaseRequest_builder{Name: name}.Build())
				return resp.GetPhase(), trace.Wrap(err)
			}, beam)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		published, err = publication.publish(cf.Context, api)
		return trace.Wrap(err)
	})
	if _, writeErr := io.Copy(cf.Stderr(), &diagnostics); writeErr != nil {
		return trace.NewAggregate(err, writeErr)
	}
	err = c.finishPublication(published, err, beam.BeamID(), func() error {
		if err := c.printPublished(cf, published); err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(cf.Stderr(), "Quitting this build.")
		cmd := exec.CommandContext(cf.Context, "/usr/local/bin/beam-abandon")
		cmd.Stdout = io.Discard
		cmd.Stderr = cf.Stderr()
		return cmd.Run()
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if c.final {
		return nil
	}
	return c.printPublished(cf, published)
}

func (c *phasePublishCommand) printPublished(cf *CLIConf, published *beamsv1.Phase) error {
	if published.GetStatus().GetStrategy() == beamsv1.PhaseStrategy_PHASE_STRATEGY_STATIC {
		fmt.Fprintln(cf.Stderr(), "Warning: this snapshot is static. Static Phases are insecure and never automatically rebuilt. New Phases cannot be derived from a static Phase.")
	}
	switch c.format {
	case teleport.JSON:
		return trace.Wrap(toolcommon.PrintJSONIndent(cf.Stdout(), formatPhase(published)))
	case teleport.YAML:
		return trace.Wrap(toolcommon.PrintYAML(cf.Stdout(), formatPhase(published)))
	default:
		name := published.GetMetadata().GetName()
		_, err := fmt.Fprintf(cf.Stdout(),
			"Saved snapshot %s to Phase %q.\n"+
				"Start a Beam from the latest snapshot: tsh beams add --phase=%s\n",
			published.GetStatus().GetPublished().GetDigest(), name, name)
		if err != nil || published.GetSpec().GetBuildConfig() != "" {
			return trace.Wrap(err)
		}
		if published.GetStatus().GetStrategy() == beamsv1.PhaseStrategy_PHASE_STRATEGY_COPY {
			_, err = fmt.Fprintln(cf.Stdout(),
				"Replace its copied files using --copy. Interactive editing and forking are not supported.")
			return trace.Wrap(err)
		}
		_, err = fmt.Fprintf(cf.Stdout(),
			"Fork it into a new Phase: tsh phase add --fork-from=%s\n"+
				"Edit it in-place (if no dependents): tsh phase edit %s\n", name, name)
		return trace.Wrap(err)
	}
}

// A transport or internal error can occur after Auth has accepted publication.
// Preserve specific rejection errors and the normal authentication retry behavior.
func phasePublicationError(err error) error {
	if trace.IsBadParameter(err) ||
		trace.IsAccessDenied(err) ||
		trace.IsNotFound(err) ||
		trace.IsAlreadyExists(err) ||
		trace.IsCompareFailed(err) ||
		trace.IsLimitExceeded(err) ||
		client.IsErrorResolvableWithRelogin(err) {
		return trace.Wrap(err)
	}
	return trace.Wrap(err, "snapshot may still complete. Check tsh phases ls before retrying")
}

// Validate both before snapshotting and against the publication response so a
// stale cache entry cannot cause a different build to be abandoned.
func validatePhaseFinalBuild(phase *beamsv1.Phase, beamID string) error {
	if beamID == "" ||
		phase.GetStatus().GetOperation().GetBuildBeam() != beamID ||
		phase.GetStatus().GetState() != beamsv1.PhaseBuildState_PHASE_BUILD_STATE_BUILDING ||
		phase.GetSpec().GetBuildConfig() != "" {
		return trace.BadParameter("--final must run inside the target Phase's active interactive build Beam")
	}
	return nil
}

// Abandon only a confirmed publication, outside authentication retries. A failed
// or uncertain publication must leave the build available to the user.
func (c *phasePublishCommand) finishPublication(published *beamsv1.Phase, publishErr error, beamID string, abandon func() error) error {
	if publishErr != nil {
		return trace.Wrap(publishErr)
	}
	if !c.final {
		return nil
	}
	if err := validatePhaseFinalBuild(published, beamID); err != nil {
		return trace.Wrap(err, "snapshot published but the active build changed. The local build was not ended")
	}
	if err := abandon(); err != nil {
		return trace.Wrap(err, "snapshot saved, but quitting the build failed")
	}
	return nil
}
