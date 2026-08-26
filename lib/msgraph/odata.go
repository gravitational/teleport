// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package msgraph

import "encoding/json"

// oDataPage defines the structure of a response to a paginated MS Graph endpoint.
// Value is an abstract `json.RawMessage` type to offer flexibility for the consumer,
// e.g. [client.IterateGroupMembers] will deserialize each of the array elements into potentially different concrete types.
type oDataPage struct {
	NextLink string          `json:"@odata.nextLink,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
}

// oDataListResponse defines the structure of a simple "list" response from the MS Graph API.
type oDataListResponse[T any] struct {
	Value []T `json:"value,omitempty"`
}
