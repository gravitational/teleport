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

package services

import (
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

const (
	// MaxPhaseBuildConfigSize keeps Phase resources small enough to distribute
	// safely through the protobuf and cache watch paths.
	MaxPhaseBuildConfigSize = 32 * 1024

	// DefaultPhaseName selects the standard Beam image without looking up a
	// stored Phase resource.
	DefaultPhaseName = "default:v1"
)

// ParsePhaseSnapshot splits a snapshot reference into its parent Phase name
// and image digest. The reference identifies the image used as a build input,
// not the Phase that will receive the new image.
func ParsePhaseSnapshot(ref string) (string, string, error) {
	parent, digest, ok := strings.Cut(ref, "@")
	if !ok ||
		parent == "" ||
		!IsValidImageDigest(digest) {
		return "", "", trace.BadParameter("snapshot must be parent@sha256:<64 lowercase hex digits>")
	}
	return parent, digest, nil
}

var imageDigestRegexp = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// IsValidImageDigest reports whether digest is a canonical sha256 OCI digest.
func IsValidImageDigest(digest string) bool {
	return imageDigestRegexp.MatchString(digest)
}
