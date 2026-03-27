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

import { Regions as AwsRegion } from 'teleport/services/integrations';

import { WildcardRegion } from '../Shared';

export type AwsLabel = {
  name: string;
  value: string;
};

export type ServiceConfig = {
  enabled: boolean;
  regions: WildcardRegion | AwsRegion[];
  tags: AwsLabel[];
};

export type ServiceType = 'ec2' | 'eks';

export type ServiceConfigs = Record<ServiceType, ServiceConfig>;

export const serviceTypes: ServiceType[] = ['ec2', 'eks'];
