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

package resources

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func collectionFormatTest(t *testing.T, collection Collection, wantVerbose, wantNonVerbose string) {
	t.Helper()

	t.Run("verbose mode", func(t *testing.T) {
		t.Helper()
		w := &bytes.Buffer{}
		err := collection.WriteText(w, true)
		require.NoError(t, err)
		diff := cmp.Diff(wantVerbose, w.String())
		require.Empty(t, diff)
	})

	t.Run("non-verbose mode", func(t *testing.T) {
		t.Helper()
		w := &bytes.Buffer{}
		err := collection.WriteText(w, false)
		require.NoError(t, err)
		diff := cmp.Diff(wantNonVerbose, w.String())
		require.Empty(t, diff)
	})
}
