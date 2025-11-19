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

import { Regions } from 'teleport/services/integrations';

// Wildcard region type for "all regions" selection
export type WildcardRegion = ['*'];

// Base service configuration (common fields)
export type BaseServiceConfig = {
  enabled: boolean;
  tags: AwsLabel[];
};

// Service-specific configurations
export type Ec2Config = BaseServiceConfig;

export type RdsConfig = BaseServiceConfig;

export type EksConfig = BaseServiceConfig & {
  enableAppDiscovery: boolean;
};

export type IntegrationConfig = {
  name: string;
  roleArn: string;
};

export type AwsConfig = {
  integration: IntegrationConfig;
  accountId: string;
  regions: WildcardRegion | Regions[];
};

export type AwsLabel = {
  name: string;
  value: string;
};
