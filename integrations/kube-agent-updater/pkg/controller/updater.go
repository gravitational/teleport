/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/gravitational/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/img"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/maintenance"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/version"
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
		return nil, &NoNewVersionError{CurrentVersion: currentVersion, NextVersion: nextVersion}
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
