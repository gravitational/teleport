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
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	rsstypes "github.com/aws/aws-sdk-go-v2/service/redshiftserverless/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/gravitational/teleport/api/types"
)

// ResourceTag is a generic interface that represents an AWS resource tag.
type ResourceTag interface {
	// TODO Go generic does not allow access common fields yet. List all types
	//  here and use a type switch for now.
	rdstypes.Tag |
		ec2types.Tag |
		redshifttypes.Tag |
		ectypes.Tag |
		memorydbtypes.Tag |
		rsstypes.Tag |
		opensearchtypes.Tag |
		smtypes.Tag
}

// TagsToLabels converts a list of AWS resource tags to a label map.
func TagsToLabels[Tag ResourceTag](tags []Tag) map[string]string {
	if len(tags) == 0 {
		return nil
	}

	labels := make(map[string]string)
	for _, tag := range tags {
		key, value := resourceTagToKeyValue(tag)

		if types.IsValidLabelKey(key) {
			labels[key] = value
		} else {
			slog.DebugContext(context.Background(), "Skipping AWS resource tag with invalid label key", "key", key)
		}
	}
	return labels
}

func resourceTagToKeyValue[Tag ResourceTag](tag Tag) (string, string) {
	switch v := any(tag).(type) {
	case ectypes.Tag:
		return aws.ToString(v.Key), aws.ToString(v.Value)
	case memorydbtypes.Tag:
		return aws.ToString(v.Key), aws.ToString(v.Value)
	case rsstypes.Tag:
		return aws.ToString(v.Key), aws.ToString(v.Value)
	case rdstypes.Tag:
		return aws.ToString(v.Key), aws.ToString(v.Value)
	case ec2types.Tag:
		return aws.ToString(v.Key), aws.ToString(v.Value)
	case redshifttypes.Tag:
		return aws.ToString(v.Key), aws.ToString(v.Value)
	case opensearchtypes.Tag:
		return aws.ToString(v.Key), aws.ToString(v.Value)
	case smtypes.Tag:
		return aws.ToString(v.Key), aws.ToString(v.Value)
	default:
		return "", ""
	}
}
