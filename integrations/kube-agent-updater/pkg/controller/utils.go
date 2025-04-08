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
	"strconv"

	"github.com/distribution/reference"
	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

func getWorkloadVersion(podSpec v1.PodSpec) (string, error) {
	var current string
	image, err := getContainerImageFromPodSpec(podSpec, teleportContainerName)
	if err != nil {
		return current, trace.Wrap(err)
	}

	imageRef, err := reference.ParseNamed(image)
	if err != nil {
		return current, trace.Wrap(err)
	}
	taggedImageRef, ok := imageRef.(reference.Tagged)
	if !ok {
		return "", trace.BadParameter("imageRef %s is not tagged", imageRef)
	}
	current = taggedImageRef.Tag()
	current, err = version.EnsureSemver(current)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return current, nil
}

func getContainerImageFromPodSpec(spec v1.PodSpec, container string) (string, error) {
	for _, containerSpec := range spec.Containers {
		if containerSpec.Name == container {
			return containerSpec.Image, nil
		}
	}
	return "", trace.NotFound("container %q not found in podSpec", container)
}

func setContainerImageFromPodSpec(spec *v1.PodSpec, container, image string) error {
	for i, containerSpec := range spec.Containers {
		if containerSpec.Name == container {
			spec.Containers[i].Image = image
			return nil
		}
	}
	return trace.NotFound("container %q not found in podSpec", container)
}

// skipReconciliation checks if the object has an annotation specifying that we
// must skip the reconciliation. Disabling reconciliation is useful for
// debugging purposes or when the user wants to suspend the updater for some
// reason.
func skipReconciliation(object metav1.Object) bool {
	annotations := object.GetAnnotations()
	if reconciliationAnnotation, ok := annotations[skipReconciliationAnnotation]; ok {
		skip, err := strconv.ParseBool(reconciliationAnnotation)
		if err != nil {
			return false
		}
		return skip
	}
	return false
}
