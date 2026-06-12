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

package aws

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/aws"
)

func TestGetKnownRegions(t *testing.T) {
	// Picked a few regions just to make sure GetKnownRegions is returning
	// something that includes these.
	t.Run("hand picked", func(t *testing.T) {
		for _, region := range []string{
			"cn-north-1",
			"eu-isoe-west-1",
			"il-central-1",
			"us-east-1",
			"us-gov-west-1",
			"us-isob-east-1",
			"us-isob-east-1",
		} {
			require.Contains(t, GetKnownRegions(), region)
		}
	})

	// Ideally this should be tested in api/utils/aws but api has no access
	// to AWS SDK. If this fails aws.IsValidRegion should be updated.
	t.Run("IsValidRegion", func(t *testing.T) {
		for _, region := range GetKnownRegions() {
			require.NoError(t, aws.IsValidRegion(region))
		}
	})

}
func TestIsKnownRegion(t *testing.T) {
	for _, region := range GetKnownRegions() {
		require.True(t, IsKnownRegion(region))
	}

	for _, region := range []string{
		"us-east-100",
		"cn-north",
	} {
		require.False(t, IsKnownRegion(region))
	}
}
