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

package common

import (
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// GetServiceNamesFromAnnotations reads systems annotations from an access
// requests and returns the services to notify/use for approval.
// The list is sorted and duplicates are removed.
func GetServiceNamesFromAnnotations(req types.AccessRequest, annotationKey string) ([]string, error) {
	serviceNames, ok := req.GetSystemAnnotations()[annotationKey]
	if !ok {
		return nil, trace.NotFound("request annotation %s is missing", annotationKey)
	}
	if len(serviceNames) == 0 {
		return nil, trace.BadParameter("request annotation %s is present but empty", annotationKey)
	}
	slices.Sort(serviceNames)
	return slices.Compact(serviceNames), nil
}
