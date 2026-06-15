/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

const regions: { name: string; regions: AzureRegion[] }[] = [
  {
    name: 'Americas',
    regions: [
      'brazilsouth',
      'brazilsoutheast',
      'canadacentral',
      'canadaeast',
      'centralus',
      'chilecentral',
      'eastus',
      'eastus2',
      'mexicocentral',
      'northcentralus',
      'southcentralus',
      'westcentralus',
      'westus',
      'westus2',
      'westus3',
    ],
  },

  {
    name: 'Europe',
    regions: [
      'austriaeast',
      'belgiumcentral',
      'denmarkeast',
      'francecentral',
      'francesouth',
      'germanynorth',
      'germanywestcentral',
      'italynorth',
      'northeurope',
      'norwayeast',
      'norwaywest',
      'polandcentral',
      'spaincentral',
      'swedencentral',
      'switzerlandnorth',
      'switzerlandwest',
      'uksouth',
      'ukwest',
      'westeurope',
    ],
  },
  {
    name: 'Asia Pacific',
    regions: [
      'australiacentral',
      'australiacentral2',
      'australiaeast',
      'australiasoutheast',
      'centralindia',
      'eastasia',
      'indonesiacentral',
      'japaneast',
      'japanwest',
      'koreacentral',
      'koreasouth',
      'malaysiawest',
      'newzealandnorth',
      'southeastasia',
      'southindia',
      'westindia',
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

export const azureRegionOptionGroups = regions.map(({ name, regions }) => ({
  label: name,
  options: regions
    .filter(id => id in azureRegionMap)
    .map(id => ({
      value: id,
      label: azureRegionMap[id],
    })),
}));

export const azureRegionOptions = azureRegionOptionGroups.flatMap(
  group => group.options
);
