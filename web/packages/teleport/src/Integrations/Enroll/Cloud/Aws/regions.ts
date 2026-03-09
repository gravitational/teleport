/**
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

import { awsRegionMap } from 'teleport/services/integrations';

import { Region, RegionGroup, RegionId } from '../RegionMultiSelector/types';

const regions: { name: string; regions: RegionId[] }[] = [
  {
    name: 'North America',
    regions: [
      'us-east-1',
      'us-east-2',
      'us-west-1',
      'us-west-2',
      'ca-central-1',
    ],
  },
  {
    name: 'South America',
    regions: ['sa-east-1'],
  },
  {
    name: 'Asia Pacific',
    regions: [
      'ap-east-1',
      'ap-northeast-1',
      'ap-northeast-2',
      'ap-northeast-3',
      'ap-south-1',
      'ap-south-2',
      'ap-southeast-1',
      'ap-southeast-2',
      'ap-southeast-3',
      'ap-southeast-4',
    ],
  },
  {
    name: 'Europe',
    regions: [
      'eu-central-1',
      'eu-central-2',
      'eu-north-1',
      'eu-south-1',
      'eu-south-2',
      'eu-west-1',
      'eu-west-2',
      'eu-west-3',
    ],
  },
  {
    name: 'Middle East',
    regions: ['me-south-1', 'me-central-1', 'il-central-1'],
  },
  {
    name: 'Africa',
    regions: ['af-south-1'],
  },
];

function createAwsRegionGroups(): readonly RegionGroup[] {
  return regions.map(({ name, regions }) => ({
    name,
    regions: regions
      .filter(id => id in awsRegionMap)
      .map(id => ({
        id,
        name: awsRegionMap[id],
      })) as Region[],
  }));
}

export const awsRegionGroups = createAwsRegionGroups();
