/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';

export function getResourceType(type: string): AwsResource {
  switch (type) {
    case 'ec2':
    case 'discover-ec2':
      return AwsResource.ec2;
    case 'eks':
    case 'discover-eks':
      return AwsResource.eks;
    case 'rds':
    case 'discover-rds':
    default:
      return AwsResource.rds;
  }
}
