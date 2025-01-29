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

	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	rsstypes "github.com/aws/aws-sdk-go-v2/service/redshiftserverless/types"
)

func LabelsToTags[T any](labels map[string]string, convert func(string, string) T) []T {
	keys := slices.Sorted(maps.Keys(labels))

	ret := make([]T, 0, len(keys))
	for _, key := range keys {
		value := labels[key]

		ret = append(ret, convert(key, value))
	}
	return ret
}

// LabelsToRedshiftTags converts labels into [redshifttypes.Tag] list.
func LabelsToRedshiftTags(labels map[string]string) []redshifttypes.Tag {
	return LabelsToTags(labels, func(key, value string) redshifttypes.Tag {
		return redshifttypes.Tag{Key: &key, Value: &value}
	})
}

// LabelsToRDSTags converts labels into a [rdstypes.Tag] list.
func LabelsToRDSTags(labels map[string]string) []rdstypes.Tag {
	return LabelsToTags(labels, func(key, value string) rdstypes.Tag {
		return rdstypes.Tag{Key: &key, Value: &value}
	})
}

// LabelsToRedshiftServerlessTags converts labels into a [rsstypes.Tag] list.
func LabelsToRedshiftServerlessTags(labels map[string]string) []rsstypes.Tag {
	return LabelsToTags(labels, func(key, value string) rsstypes.Tag {
		return rsstypes.Tag{Key: &key, Value: &value}
	})
}

// LabelsToElastiCacheTags converts labels into a [ectypes.Tag] list.
func LabelsToElastiCacheTags(labels map[string]string) []ectypes.Tag {
	return LabelsToTags(labels, func(key, value string) ectypes.Tag {
		return ectypes.Tag{Key: &key, Value: &value}
	})
}

// LabelsToMemoryDBTags converts labels into a [memorydbtypes.Tag] list.
func LabelsToMemoryDBTags(labels map[string]string) []memorydbtypes.Tag {
	return LabelsToTags(labels, func(key, value string) memorydbtypes.Tag {
		return memorydbtypes.Tag{Key: &key, Value: &value}
	})
}

// LabelsToOpenSearchTags converts labels into a [opensearchtypes.Tag] list.
func LabelsToOpenSearchTags(labels map[string]string) []opensearchtypes.Tag {
	return LabelsToTags(labels, func(key, value string) opensearchtypes.Tag {
		return opensearchtypes.Tag{Key: &key, Value: &value}
	})
}
