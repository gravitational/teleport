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
	"time"

	"github.com/alecthomas/kingpin/v2"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
)

type phaseCommands struct {
	publish  *phasePublishCommand
	add      *phaseAddCommand
	apply    *phaseApplyCommand
	edit     *phaseEditCommand
	ls       *phaseLSCommand
	lsFailed *phaseLSCommand
	rm       *phaseRMCommand
}

func newPhaseCommands(app *kingpin.Application) phaseCommands {
	parent := app.Command("phase", "Create and manage reusable Beam images.").Alias("phases")
	return phaseCommands{
		publish:  newPhasePublishCommand(parent),
		add:      newPhaseAddCommand(parent),
		apply:    newPhaseApplyCommand(parent),
		edit:     newPhaseEditCommand(parent),
		ls:       newPhaseLSCommand(parent),
		lsFailed: newPhaseFailedLSCommand(parent),
		rm:       newPhaseRMCommand(parent),
	}
}

type formattedPhase struct {
	Name         string                 `json:"name"`
	Owner        string                 `json:"owner"`
	Strategy     string                 `json:"strategy"`
	Parent       string                 `json:"parent,omitempty"`
	State        string                 `json:"state"`
	BuildBeam    string                 `json:"build_beam,omitempty"`
	ImageDigest  string                 `json:"image_digest,omitempty"`
	Region       string                 `json:"region,omitempty"`
	Roles        []string               `json:"roles,omitempty"`
	PublishedAt  *time.Time             `json:"published_at,omitempty"`
	NextBuild    *time.Time             `json:"next_build,omitempty"`
	LastError    string                 `json:"last_error,omitempty"`
	FailedBuilds []formattedFailedBuild `json:"failed_builds,omitempty"`
}

type formattedFailedBuild struct {
	Digest     string     `json:"digest"`
	Timestamp  *time.Time `json:"timestamp,omitempty"`
	Expiration *time.Time `json:"expiration,omitempty"`
}

func formatPhase(phase *beamsv1.Phase) formattedPhase {
	formatted := formattedPhase{
		Name:        phase.GetMetadata().GetName(),
		Owner:       phase.GetStatus().GetUser(),
		Strategy:    phaseStrategyString(phase.GetStatus().GetStrategy()),
		Parent:      phase.GetSpec().GetParent(),
		State:       phaseStateString(phase.GetStatus().GetState()),
		ImageDigest: phase.GetStatus().GetPublished().GetDigest(),
		Region:      phase.GetStatus().GetRegion(),
		Roles:       phase.GetStatus().GetDelegation().GetRoles(),
		LastError:   phase.GetStatus().GetLastError(),
	}
	if value := phase.GetStatus().GetPublished().GetPublishedAt(); value != nil {
		publishedAt := value.AsTime()
		formatted.PublishedAt = &publishedAt
	}
	if value := phase.GetStatus().GetNextBuildAt(); value != nil {
		nextBuild := value.AsTime()
		formatted.NextBuild = &nextBuild
	}
	for _, failed := range phase.GetStatus().GetFailedBuilds() {
		timestamp := failed.GetTimestamp().AsTime()
		expiration := failed.GetExpiration().AsTime()
		formatted.FailedBuilds = append(formatted.FailedBuilds, formattedFailedBuild{
			Digest:     failed.GetDigest(),
			Timestamp:  &timestamp,
			Expiration: &expiration,
		})
	}
	return formatted
}

func phaseStrategyString(strategy beamsv1.PhaseStrategy) string {
	switch strategy {
	case beamsv1.PhaseStrategy_PHASE_STRATEGY_STATIC:
		return "static"
	case beamsv1.PhaseStrategy_PHASE_STRATEGY_REBUILD:
		return "rebuild"
	case beamsv1.PhaseStrategy_PHASE_STRATEGY_SNAPSHOT:
		return "snapshot"
	case beamsv1.PhaseStrategy_PHASE_STRATEGY_COPY:
		return "copy"
	default:
		return "unknown"
	}
}

func phaseStateString(state beamsv1.PhaseBuildState) string {
	switch state {
	case beamsv1.PhaseBuildState_PHASE_BUILD_STATE_BUILDING:
		return "building"
	case beamsv1.PhaseBuildState_PHASE_BUILD_STATE_READY:
		return "ready"
	case beamsv1.PhaseBuildState_PHASE_BUILD_STATE_FAILED:
		return "failed"
	case beamsv1.PhaseBuildState_PHASE_BUILD_STATE_DELETING:
		return "deleting"
	case beamsv1.PhaseBuildState_PHASE_BUILD_STATE_BLOCKED:
		return "blocked"
	default:
		return "unknown"
	}
}
