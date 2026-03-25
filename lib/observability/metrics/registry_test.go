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

package metrics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWrap(t *testing.T) {
	testNamespace := "namespace"
	tests := []struct {
		name              string
		existingSubsystem string
		wrappingSubsystem string
		expectedSubsystem string
	}{
		{
			name:              "empty subsystem + empty subsystem",
			existingSubsystem: "",
			wrappingSubsystem: "",
			expectedSubsystem: "",
		},
		{
			name:              "empty subsystem + non-empty subsystem",
			existingSubsystem: "",
			wrappingSubsystem: "test",
			expectedSubsystem: "test",
		},
		{
			name:              "non-empty subsystem + empty subsystem",
			existingSubsystem: "test",
			wrappingSubsystem: "",
			expectedSubsystem: "test",
		},
		{
			name:              "non-empty subsystem + non-empty subsystem",
			existingSubsystem: "test",
			wrappingSubsystem: "test",
			expectedSubsystem: "test_test",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := &Registry{
				Registerer: nil,
				namespace:  testNamespace,
				subsystem:  test.existingSubsystem,
			}
			result := r.Wrap(test.wrappingSubsystem)
			require.Equal(t, test.expectedSubsystem, result.subsystem)
			require.Equal(t, testNamespace, result.namespace)
		})
	}
}
