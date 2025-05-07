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

	"github.com/gravitational/teleport/api/types"
)

// newHeadlessAuthenticationCollection creates a collection that does not
// fetch from upstream and does not store the entire set of resources.
// The collection is only required so that watchers can be created and
// events can be processed.
func newHeadlessAuthenticationCollection(w types.WatchKind) (*collection[*types.HeadlessAuthentication, string], error) {
	return &collection[*types.HeadlessAuthentication, string]{
		store: newStore(map[string]func(*types.HeadlessAuthentication) string{
			"default": func(ha *types.HeadlessAuthentication) string {
				return "default"
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*types.HeadlessAuthentication, error) {
			return nil, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *types.HeadlessAuthentication {
			return &types.HeadlessAuthentication{
				ResourceHeader: types.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
			}
		},
		watch: w,
	}, nil
}
