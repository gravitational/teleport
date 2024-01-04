/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package controller

import (
	"context"
	"strings"

	"github.com/distribution/reference"
	"github.com/gravitational/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/img"
	"github.com/gravitational/teleport/lib/automaticupgrades/maintenance"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

type VersionUpdater struct {
	versionGetter       version.Getter
	imageValidators     img.Validators
	maintenanceTriggers maintenance.Triggers
	baseImage           reference.Named
}

// GetVersion does all the version update logic: checking if a maintenance is allowed,
// retrieving the new version, comparing it with the current version and
// validating the new image signature.
// If all steps are successfully executed and there's a new version, it returns
// a digested reference to the new image that should be deployed.
func (r *VersionUpdater) GetVersion(ctx context.Context, obj client.Object, currentVersion string) (img.NamedTaggedDigested, error) {
	// Those are debug logs only
	log := ctrllog.FromContext(ctx).V(1)

	// Can we do a maintenance?
	log.Info("Checking if a maintenance trigger is on")
	if !r.maintenanceTriggers.CanStart(ctx, obj) {
		return nil, &MaintenanceNotTriggeredError{}
	}
	log.Info("Maintenance triggered, getting new version")

	// Get the next version
	nextVersion, err := r.versionGetter.GetVersion(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Info("New version candidate", "nextVersion", nextVersion)
	if !version.ValidVersionChange(ctx, currentVersion, nextVersion) {
		return nil, &version.NoNewVersionError{CurrentVersion: currentVersion, NextVersion: nextVersion}
	}

	log.Info("Version change is valid, building img candidate")
	// We tag our img candidate with the version
	image, err := reference.WithTag(r.baseImage, strings.TrimPrefix(nextVersion, "v"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Info("Verifying candidate img", "img", image.String())
	// We validate the signatures
	digested, err := r.imageValidators.Validate(ctx, image)
	if err != nil {
		return nil, trace.Trust(err, "no valid signature found for img %s", image)
	}

	log.Info("The following image was verified", "verifiedImage", digested.String())
	return digested, nil
}

// NewVersionUpdater returns a version updater using the given version.Getter,
// img.Validators, maintenance.Triggers and baseImage.
func NewVersionUpdater(v version.Getter, i img.Validators, t maintenance.Triggers, b reference.Named) VersionUpdater {
	// TODO: do checks to see if not nil/empty ?
	return VersionUpdater{
		versionGetter:       v,
		imageValidators:     i,
		maintenanceTriggers: t,
		baseImage:           b,
	}
}
