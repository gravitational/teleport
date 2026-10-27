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
	"cmp"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/services"
)

type phaseEnvironment interface {
	PhaseName() (string, error)
	BeamID() string
	// CreateSnapshot returns parent@sha256:digest without surrounding whitespace.
	CreateSnapshot(context.Context) (string, error)
}

type buildBeamEnvironment struct {
	phaseName    string
	beamID       string
	stderr       io.Writer
	snapshotArgs []string
}

func localBuildBeam(stderr io.Writer) buildBeamEnvironment {
	return buildBeamEnvironment{
		phaseName: os.Getenv("BEAM_PHASE_NAME"),
		beamID:    os.Getenv("BEAM_ID"),
		stderr:    stderr,
	}
}

// PhaseName returns the local build Beam's Phase or an error outside a build Beam.
func (b buildBeamEnvironment) PhaseName() (string, error) {
	if b.phaseName == "" {
		return "", trace.BadParameter("not running in a build Beam")
	}
	return b.phaseName, nil
}

// BeamID returns the local Beam's identifier, or an empty string outside a Beam.
func (b buildBeamEnvironment) BeamID() string {
	return b.beamID
}

// CreateSnapshot captures the local filesystem and returns its trimmed snapshot reference.
func (b buildBeamEnvironment) CreateSnapshot(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "/usr/local/bin/beam-snapshot", b.snapshotArgs...)
	cmd.Stderr = b.stderr
	output, err := cmd.Output()
	return strings.TrimSpace(string(output)), err
}

func (c *phaseAddCommand) snapshotSource(ctx context.Context, env phaseEnvironment) (string, error) {
	if err := c.validateSnapshotSource(); err != nil {
		return "", trace.Wrap(err)
	}
	if !c.forkLocal && !c.forkFromSet {
		return "", nil
	}
	ref := c.forkSource
	if c.forkLocal {
		if _, err := env.PhaseName(); err != nil {
			return "", trace.BadParameter(
				"--fork requires a build Beam. Otherwise supply --fork-from=Phase or --fork-from=parent@sha256:digest",
			)
		}
		snapshot, err := env.CreateSnapshot(ctx)
		if err != nil {
			return "", trace.Wrap(err, "running local beam-snapshot")
		}
		ref = snapshot
	}
	if c.forkLocal ||
		ref == "" ||
		strings.Contains(ref, "@") ||
		strings.HasPrefix(ref, "sha256:") {
		if _, _, err := services.ParsePhaseSnapshot(ref); err != nil {
			return "", trace.Wrap(err)
		}
	}
	// A named source is resolved after connecting to Auth.
	return ref, nil
}

func (c *phaseAddCommand) validateSnapshotSource() error {
	if c.forkLocal && c.forkFromSet {
		return trace.BadParameter("--fork and --fork-from are mutually exclusive")
	}
	if !c.forkLocal && !c.forkFromSet {
		return nil
	}
	if c.fromSet || c.commandsFile != "" || len(c.copies) != 0 {
		return trace.BadParameter("--fork and --fork-from cannot be combined with --from, --commands-file, or --copy")
	}
	return nil
}

func (c *phaseEditCommand) snapshotSource() (string, error) {
	if (c.commandsFile != "" || len(c.copies) != 0) && c.fromSet {
		return "", trace.BadParameter("--commands-file or --copy cannot be combined with --from when editing")
	}
	if c.fromSet && c.editSource == "" {
		return "", trace.BadParameter("--from requires a Phase name or parent@sha256:digest")
	}
	if c.commandsFile != "" || len(c.copies) != 0 {
		return "", nil // Auth preserves the existing parent for command and file edits.
	}
	return cmp.Or(c.editSource, c.name), nil
}

type phaseSnapshotSource struct {
	parent string
	digest string
	region string
}

// resolvePhaseSnapshot resolves the image and registry placement of a snapshot.
// Explicit references have no source Phase from which to determine the region.
func resolvePhaseSnapshot(
	ctx context.Context,
	ref string,
	get func(context.Context, string) (*beamsv1.Phase, error),
) (phaseSnapshotSource, error) {
	if ref == "" {
		return phaseSnapshotSource{}, nil
	}
	if strings.Contains(ref, "@") {
		parent, digest, err := services.ParsePhaseSnapshot(ref)
		return phaseSnapshotSource{
			parent: parent,
			digest: digest,
		}, trace.Wrap(err)
	}
	if strings.HasPrefix(ref, "sha256:") {
		return phaseSnapshotSource{}, trace.BadParameter("snapshot must be parent@sha256:digest")
	}
	source, err := get(ctx, ref)
	if err != nil {
		return phaseSnapshotSource{}, trace.Wrap(err, "resolving snapshot source Phase %q", ref)
	}
	if source.GetStatus().GetStrategy() == beamsv1.PhaseStrategy_PHASE_STRATEGY_COPY {
		return phaseSnapshotSource{}, trace.BadParameter("cannot fork copy Phase %q. Use --from to build on its image", ref)
	}
	digest := source.GetStatus().GetPublished().GetDigest()
	if !services.IsValidImageDigest(digest) {
		return phaseSnapshotSource{}, trace.BadParameter("source Phase %q has no successful snapshot", ref)
	}
	parent := source.GetSpec().GetParent()
	if parent == "" {
		return phaseSnapshotSource{}, trace.BadParameter("source Phase %q has no parent", ref)
	}
	if _, _, err := services.ParsePhaseSnapshot(parent + "@" + digest); err != nil {
		return phaseSnapshotSource{}, trace.Wrap(err)
	}
	return phaseSnapshotSource{
		parent: parent,
		digest: digest,
		region: source.GetStatus().GetRegion(),
	}, nil
}
