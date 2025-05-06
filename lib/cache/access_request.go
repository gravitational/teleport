// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func newAccessRequestCollection(upstream services.DynamicAccessCore, w types.WatchKind) (*collection[types.AccessRequest, string], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter DynamicAccess")
	}

	return &collection[types.AccessRequest, string]{
		store: newStore(map[string]func(types.AccessRequest) string{
			"default": func(types.AccessRequest) string { return "default" },
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.AccessRequest, error) {
			return nil, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.AccessRequest {
			return &types.AccessRequestV3{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.GetName(),
				},
			}
		},
		watch: w,
	}, nil
}
