/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package version

import (
	"context"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/mod/semver"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// Getter gets the target image version for an external source. It should cache
// the result to reduce io and avoid potential rate-limits and is safe to call
// multiple times over a short period.
// If the version source intentionally returns no version, a NoNewVersionError is
// returned.
type Getter interface {
	GetVersion(context.Context) (string, error)
}

// FailoverGetter wraps multiple Getters and tries them sequentially.
// Any error is considered fatal, except for the trace.NotImplementedErr
// which indicates the version getter is not supported yet and we should
// failover to the next version getter.
type FailoverGetter []Getter

// GetVersion implements Getter
// Getters are evaluated sequentially, the result of the first getter not returning
// trace.NotImplementedErr is used.
func (f FailoverGetter) GetVersion(ctx context.Context) (string, error) {
	for _, getter := range f {
		version, err := getter.GetVersion(ctx)
		switch {
		case err == nil:
			return version, nil
		case trace.IsNotImplemented(err):
			continue
		default:
			return "", trace.Wrap(err)
		}
	}
	return "", trace.NotFound("every versionGetter returned NotImplemented")
}

// ValidVersionChange receives the current version and the candidate next version
// and evaluates if the version transition is valid.
func ValidVersionChange(ctx context.Context, current, next string) bool {
	log := ctrllog.FromContext(ctx).V(1)
	// Cannot upgrade to a non-valid version
	if !semver.IsValid(next) {
		log.Error(
			trace.BadParameter("next version is not following semver"),
			"version change is invalid",
			"current_version", current,
			"next_version", next,
		)
		return false
	}
	switch semver.Compare(next, current) {
	// No need to upgrade if version is the same
	case 0:
		return false
	default:
		return true
	}
}

// EnsureSemver adds the 'v' prefix if needed and ensures the provided version
// is semver-compliant.
func EnsureSemver(current string) (string, error) {
	if !strings.HasPrefix(current, "v") {
		current = "v" + current
	}
	if !semver.IsValid(current) {
		return "", trace.BadParameter("tag %s is not following semver", current)
	}
	return current, nil
}
