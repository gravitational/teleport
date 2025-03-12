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

package generic

import (
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

// UnmarshalableResource represents a resource that can have all the necessary fields from backend.Item
// set in a generic context.
type UnmarshalableResource interface {
	SetExpiry(time.Time)
	SetRevision(string)
}

// FastUnmarshal is a generic helper used to unmarshal a resoruce from a backend.Item and
// set the Expiry and Revision fields.  It isn't compatible with the standard Unmarshal function
// signature used elsewhere and therefore may not be the best choice for all use cases, but it
// has the benefit of being simpler to use and not requiring the caller to undergo the revision/expiry
// ceremony at each call site.
func FastUnmarshal[T UnmarshalableResource](item backend.Item) (T, error) {
	var r T
	if err := utils.FastUnmarshal(item.Value, &r); err != nil {
		return r, err
	}

	r.SetExpiry(item.Expires)
	r.SetRevision(item.Revision)
	return r, nil
}

type MarshalableResource interface {
	Expiry() time.Time
	GetRevision() string
}

// FastMarshal is a generic helper used to marshal a resource to a backend.Item and
// set the Expiry and Revision fields.  It isn't compatible with the standard Marshal function
// signature used elsewhere and therefore may not be the best choice for all use cases, but it
// has the benefit of being simpler to use and not requiring the caller to undergo the revision/expiry
// ceremony at each call site.
func FastMarshal[T MarshalableResource](key backend.Key, r T) (backend.Item, error) {
	value, err := utils.FastMarshal(r)
	if err != nil {
		return backend.Item{}, err
	}

	return backend.Item{
		Key:      key,
		Value:    value,
		Expires:  r.Expiry(),
		Revision: r.GetRevision(),
	}, nil
}
