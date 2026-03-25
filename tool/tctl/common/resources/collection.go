/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package resources

import (
	"io"

	"github.com/gravitational/teleport/api/types"
)

// Collection represents a collection of resources.
// It can contain zero, one, or many resources.
// Most Collection implementation contain a single kind of resource,
// but some might return resources of mixed kinds.
type Collection interface {
	WriteText(w io.Writer, verbose bool) error
	Resources() []types.Resource
}
