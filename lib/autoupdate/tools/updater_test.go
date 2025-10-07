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

package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/types/autoupdate"
)

// TestValidateUpdateByStrategy verifies strategies upgrade/downgrade cancellation.
func TestValidateUpdateByStrategy(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name             string
		strategies       []string
		requestedVersion string
		localVersion     string
		err              error
		errMessage       string
	}{
		{
			name:             "invalid-requested-version",
			strategies:       []string{},
			requestedVersion: "100.0dev",
			localVersion:     "1.0.0",
			errMessage:       "100.0dev is not in dotted-tri format",
		},
		{
			name:             "invalid-local-version",
			strategies:       []string{},
			requestedVersion: "1.0.0",
			localVersion:     "100.0dev",
			errMessage:       "100.0dev is not in dotted-tri format",
		},
		{
			name:             "no-downgrade-restrict",
			strategies:       []string{autoupdate.ToolStrategyNoDowngrade},
			requestedVersion: "1.0.0",
			localVersion:     "3.0.0",
			err:              ErrCancelUpdate,
		},
		{
			name:             "no-downgrade-upgrade",
			strategies:       []string{autoupdate.ToolStrategyNoDowngrade},
			requestedVersion: "2.0.0",
			localVersion:     "1.0.0",
			err:              nil,
		},
		{
			name:             "ignore-major-downgrade-restrict",
			strategies:       []string{autoupdate.ToolStrategyIgnoreMajorDowngrade},
			requestedVersion: "1.0.0",
			localVersion:     "2.0.0",
			err:              ErrCancelUpdate,
		},
		{
			name:             "ignore-major-downgrade-minor-downgrade",
			strategies:       []string{autoupdate.ToolStrategyIgnoreMajorDowngrade},
			requestedVersion: "2.0.0",
			localVersion:     "2.2.0",
			err:              nil,
		},
		{
			name:             "ignore-major-downgrade-upgrade",
			strategies:       []string{autoupdate.ToolStrategyIgnoreMajorDowngrade},
			requestedVersion: "3.0.0",
			localVersion:     "2.2.0",
			err:              nil,
		},
		{
			name:             "combined-minor-downgrade",
			strategies:       []string{autoupdate.ToolStrategyNoDowngrade, autoupdate.ToolStrategyIgnoreMajorDowngrade},
			requestedVersion: "2.0.0",
			localVersion:     "2.2.0",
			err:              ErrCancelUpdate,
		},
		{
			name:             "combined-minor-patch-downgrade",
			strategies:       []string{autoupdate.ToolStrategyNoDowngrade, autoupdate.ToolStrategyIgnoreMajorDowngrade},
			requestedVersion: "2.2.0",
			localVersion:     "2.2.1",
			err:              ErrCancelUpdate,
		},
		{
			name:             "combined-upgrade",
			strategies:       []string{autoupdate.ToolStrategyNoDowngrade, autoupdate.ToolStrategyIgnoreMajorDowngrade},
			requestedVersion: "2.2.2",
			localVersion:     "2.2.1",
			err:              nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUpdateByStrategy(tt.strategies, tt.requestedVersion, tt.localVersion)
			if tt.errMessage != "" {
				assert.Contains(t, err.Error(), tt.errMessage)
			} else {
				assert.ErrorIs(t, err, tt.err)
			}
		})
	}
}
