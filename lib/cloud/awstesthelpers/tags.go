/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package awstesthelpers

import (
	"maps"
	"slices"

	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
)

// LabelsToRedshiftTags converts labels into [redshifttypes.Tag] list.
func LabelsToRedshiftTags(labels map[string]string) []redshifttypes.Tag {
	keys := slices.Collect(maps.Keys(labels))
	slices.Sort(keys)

	ret := make([]redshifttypes.Tag, 0, len(keys))
	for _, key := range keys {
		key := key
		value := labels[key]

		ret = append(ret, redshifttypes.Tag{
			Key:   &key,
			Value: &value,
		})
	}

	return ret
}
