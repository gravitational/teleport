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

package awsoidc

import (
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/lib/cloud/aws/tags"
)

// defaultResourceCreationTags returns the AWS Tags which are set on resources created by this integration.
// It will serve two purposes:
// - to identify resources created by this integration
// - when updating AWS resources, only those containing this label will be updated
func defaultResourceCreationTags(clusterName, integrationName string) tags.AWSTags {
	return tags.DefaultResourceCreationTags(clusterName, integrationName, common.OriginIntegrationAWSOIDC)
}
