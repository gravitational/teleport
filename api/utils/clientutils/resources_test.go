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

package clientutils

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
)

type mockPaginator struct {
	accessDenied bool
}

func (m *mockPaginator) List(_ context.Context, pageSize int, token string) ([]bool, string, error) {
	if m.accessDenied {
		return nil, "", trace.AccessDenied("access denied")
	}
	switch token {
	case "":
		return make([]bool, pageSize), "page1", nil
	case "page1":
		return make([]bool, pageSize), "page2", nil
	case "page2":
		return make([]bool, 5), "", nil
	default:
		return nil, "", trace.BadParameter("invalid token")
	}
}

func TestListAllResources(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		paginator := mockPaginator{}
		items, err := ListAllResources(context.Background(), paginator.List)
		require.NoError(t, err)
		require.Equal(t, defaults.DefaultChunkSize*2+5, len(items))
	})
	t.Run("error", func(t *testing.T) {
		paginator := mockPaginator{accessDenied: true}
		_, err := ListAllResources(context.Background(), paginator.List)
		require.Error(t, err)
	})
}
