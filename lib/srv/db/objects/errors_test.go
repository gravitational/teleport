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
	"errors"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestIsErrFetcherDisabled(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedMatch  bool
		expectedReason string
	}{
		{
			name:           "nil err",
			err:            nil,
			expectedMatch:  false,
			expectedReason: "",
		},
		{
			name:           "err not ErrFetcherDisabled",
			err:            errors.New("some other reason"),
			expectedMatch:  false,
			expectedReason: "",
		},
		{
			name:           "ErrFetcherDisabled with empty reason",
			err:            NewErrFetcherDisabled(""),
			expectedMatch:  true,
			expectedReason: "",
		},
		{
			name:           "ErrFetcherDisabled bare",
			err:            NewErrFetcherDisabled("dummy"),
			expectedMatch:  true,
			expectedReason: "dummy",
		},
		{
			name:           "ErrFetcherDisabled wrapped",
			err:            trace.Wrap(NewErrFetcherDisabled("dummy reason")),
			expectedMatch:  true,
			expectedReason: "dummy reason",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			match, reason := IsErrFetcherDisabled(tc.err)
			require.Equal(t, tc.expectedMatch, match)
			require.Equal(t, tc.expectedReason, reason)
		})
	}
}
