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

package events

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// This is the max size of all the events we return when searching for events.
// 1 MiB was picked as a good middleground that's commonly used by the backends and is well below
// the max size limit of a response message. This may sometimes result in an even smaller size
// when serialized as protobuf but we have no good way to check so we go by raw from each backend.
const MaxEventBytesInResponse = 1024 * 1024

func estimateEventSize(event EventFields) (int, error) {
	data, err := utils.FastMarshal(event)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return len(data), nil
}
