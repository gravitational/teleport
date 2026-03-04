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

import { azureRegionMap, AzureRegion } from 'teleport/services/integrations';

import { RegionGroup } from '../Shared/RegionMultiSelector/types';

const regions: { name: string; regions: AzureRegion[] }[] = [
  {
    name: 'North America',
    regions: [
      'eastus',
      'eastus2',
      'centralus',
      'northcentralus',
      'southcentralus',
      'westcentralus',
      'westus',
      'westus2',
      'westus3',
      'canadacentral',
      'canadaeast',
      'mexicocentral',
    ],
  },
  {
    name: 'South America',
    regions: ['brazilsouth', 'brazilsoutheast', 'chilecentral'],
  },
  {
    name: 'Europe',
    regions: [
      'northeurope',
      'westeurope',
      'uksouth',
      'ukwest',
      'francecentral',
      'francesouth',
      'germanywestcentral',
      'germanynorth',
      'norwayeast',
      'norwaywest',
      'swedencentral',
      'swedensouth',
      'switzerlandnorth',
      'switzerlandwest',
      'italynorth',
      'polandcentral',
      'spaincentral',
      'austriaeast',
      'belgiumcentral',
      'denmarkeast',
    ],
  },
  {
    name: 'Asia Pacific',
    regions: [
      'eastasia',
      'southeastasia',
      'australiaeast',
      'australiasoutheast',
      'australiacentral',
      'australiacentral2',
      'centralindia',
      'southindia',
      'westindia',
      'japaneast',
      'japanwest',
      'koreacentral',
      'koreasouth',
      'indonesiacentral',
      'malaysiawest',
      'newzealandnorth',
    ],
  },
  {
    name: 'Middle East',
    regions: ['uaenorth', 'uaecentral', 'qatarcentral', 'israelcentral'],
  },
  {
    name: 'Africa',
    regions: ['southafricanorth', 'southafricawest'],
  },
];

function createAzureRegionGroups(): readonly RegionGroup[] {
  return regions.map(({ name, regions }) => ({
    name,
    regions: regions
      .filter(id => id in azureRegionMap)
      .map(id => ({
        id,
        name: azureRegionMap[id],
      })),
  }));
}

export const azureRegionGroups = createAzureRegionGroups();
