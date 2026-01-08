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

import { useState } from 'react';

import Validation from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { Regions as AwsRegion } from 'teleport/services/integrations';

import { awsRegionGroups } from '../Aws/regions';
import { RegionMultiSelector } from './RegionMultiSelector';

export default {
  title: 'RegionMultiSelector',
  component: RegionMultiSelector,
};

export const AWS = () => {
  const [selectedRegions, setSelectedRegions] = useState<AwsRegion[]>([]);

  return (
    <Validation>
      <RegionMultiSelector
        regionGroups={awsRegionGroups}
        selectedRegions={selectedRegions}
        onChange={setSelectedRegions}
        label="Select AWS regions"
        placeholder="Select AWS regions..."
        required={true}
        rule={requiredField('At least one region is required')}
      />
    </Validation>
  );
};
