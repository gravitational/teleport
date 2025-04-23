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

package objects

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobject"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
	"github.com/gravitational/teleport/lib/utils"
)

func TestCalculateDeleted(t *testing.T) {
	tests := []struct {
		name     string
		objects  map[string]*objWithExpiry
		objsNew  map[string]*dbobjectv1.DatabaseObject
		expected []string
	}{
		{
			name:     "all deleted",
			objects:  map[string]*objWithExpiry{"a": {}, "b": {}, "c": {}},
			objsNew:  map[string]*dbobjectv1.DatabaseObject{},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "none deleted",
			objects:  map[string]*objWithExpiry{"a": {}, "b": {}, "c": {}},
			objsNew:  map[string]*dbobjectv1.DatabaseObject{"a": {}, "b": {}, "c": {}},
			expected: []string{},
		},
		{
			name:     "some deleted",
			objects:  map[string]*objWithExpiry{"a": {}, "b": {}, "c": {}},
			objsNew:  map[string]*dbobjectv1.DatabaseObject{"a": {}, "c": {}},
			expected: []string{"b"},
		},
		{
			name:     "empty input",
			objects:  map[string]*objWithExpiry{},
			objsNew:  map[string]*dbobjectv1.DatabaseObject{},
			expected: []string{},
		},
		{
			name:     "new has more keys",
			objects:  map[string]*objWithExpiry{"a": {}, "b": {}},
			objsNew:  map[string]*dbobjectv1.DatabaseObject{"a": {}, "b": {}, "c": {}},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateDeleted(context.Background(), Config{Log: slog.Default()}, tt.objects, tt.objsNew)
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestCalculateUpdates(t *testing.T) {
	clock := clockwork.NewFakeClock()

	mkObjectLabel := func(name string, label string) *dbobjectv1.DatabaseObject {
		out, err := databaseobject.NewDatabaseObjectWithLabels(name, map[string]string{"custom": label}, &dbobjectv1.DatabaseObjectSpec{
			Protocol:            types.DatabaseProtocolPostgreSQL,
			DatabaseServiceName: "dummy",
			ObjectKind:          databaseobjectimportrule.ObjectKindTable,
			Database:            "dummy",
			Schema:              "public",
			Name:                name,
		})

		require.NoError(t, err)
		return out
	}

	mkObject := func(name string) *dbobjectv1.DatabaseObject {
		return mkObjectLabel(name, "default")
	}

	tests := []struct {
		name     string
		objects  []*objWithExpiry
		objsNew  []*dbobjectv1.DatabaseObject
		expected []*objWithExpiry
	}{
		{
			name:    "all new",
			objects: []*objWithExpiry{},
			objsNew: []*dbobjectv1.DatabaseObject{
				mkObject("a"), mkObject("b"), mkObject("c"),
			},
			expected: []*objWithExpiry{
				{obj: mkObject("a"), expiry: clock.Now().Add(time.Hour)},
				{obj: mkObject("b"), expiry: clock.Now().Add(time.Hour)},
				{obj: mkObject("c"), expiry: clock.Now().Add(time.Hour)},
			},
		},
		{
			name: "none new or changed",
			objects: []*objWithExpiry{
				{obj: mkObject("a"), expiry: clock.Now().Add(time.Hour)},
				{obj: mkObject("b"), expiry: clock.Now().Add(time.Hour)},
				{obj: mkObject("c"), expiry: clock.Now().Add(time.Hour)},
			},
			objsNew: []*dbobjectv1.DatabaseObject{
				mkObject("a"), mkObject("b"), mkObject("c"),
			},
			expected: []*objWithExpiry{},
		},
		{
			name: "some changed",
			objects: []*objWithExpiry{
				{obj: mkObjectLabel("a", "old"), expiry: clock.Now().Add(time.Hour)},
				{obj: mkObject("b"), expiry: clock.Now().Add(time.Hour)},
			},
			objsNew: []*dbobjectv1.DatabaseObject{
				mkObjectLabel("a", "new"), mkObject("b"),
			},
			expected: []*objWithExpiry{
				{obj: mkObjectLabel("a", "new"), expiry: clock.Now().Add(time.Hour)},
			},
		},
		{
			name: "some refreshed",
			objects: []*objWithExpiry{
				{obj: mkObject("a"), expiry: clock.Now().Add(30 * time.Second)},
				{obj: mkObject("b"), expiry: clock.Now().Add(time.Hour)},
			},
			objsNew: []*dbobjectv1.DatabaseObject{
				mkObject("a"), mkObject("b"),
			},
			expected: []*objWithExpiry{
				{obj: mkObject("a"), expiry: clock.Now().Add(time.Hour)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				ObjectTTL:        time.Hour,
				RefreshThreshold: time.Minute,
				Log:              slog.With("test", tt.name),
				Clock:            clock,
			}

			freshObjects := utils.FromSlice(tt.objsNew, func(object *dbobjectv1.DatabaseObject) string {
				return object.GetMetadata().Name
			})

			initialState := utils.FromSlice(tt.objects, func(object *objWithExpiry) string {
				return object.obj.GetMetadata().Name
			})

			expectedState := utils.FromSlice(tt.expected, func(object *objWithExpiry) string {
				return object.obj.GetMetadata().Name
			})

			result := calculateUpdates(context.Background(), cfg, initialState, freshObjects)

			require.ElementsMatch(t, slices.Collect(maps.Keys(expectedState)), slices.Collect(maps.Keys(result)))
			for key, elem := range expectedState {
				require.Equal(t, elem.expiry, result[key].expiry)
				require.Empty(t, cmp.Diff(elem.obj, result[key].obj, protocmp.Transform()))
			}
		})
	}
}
