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

import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';

import {
  Regions as AwsRegion,
  AzureRegion,
} from 'teleport/services/integrations';

import { awsRegionOptions } from '../../Aws/regions';
import { azureRegionOptions } from '../../Azure/regions';
import { CloudRegion } from '../types';
import { RegionSelect } from './RegionSelect';

export default {
  title: 'Teleport/Integrations/Enroll/Cloud/RegionSelect',
  component: RegionSelect,
};

const requiredRegion =
  <T extends CloudRegion>(option: Option<T>) =>
  () => {
    if (!option) {
      return {
        valid: false,
        message: 'A region is required',
      };
    }
    return { valid: true };
  };

const requiredAtLeastOneRegion =
  <T extends CloudRegion>(options: Option<T>[]) =>
  () => {
    if (!options || options.length === 0) {
      return {
        valid: false,
        message: 'At least one region is required',
      };
    }
    return { valid: true };
  };

export const Aws = () => {
  const [selectedRegions, setSelectedRegions] = useState<
    readonly Option<AwsRegion>[]
  >([]);

  const [selectedRegion, setSelectedRegion] = useState<Option<AwsRegion>>(null);

  return (
    <Validation>
      <RegionSelect
        isMulti={true}
        value={selectedRegions}
        onChange={setSelectedRegions}
        options={awsRegionOptions}
        label="Select AWS regions"
        placeholder="Select AWS regions..."
        required={true}
        rule={requiredAtLeastOneRegion}
      />

      <RegionSelect
        isMulti={false}
        value={selectedRegion}
        onChange={setSelectedRegion}
        options={awsRegionOptions}
        label="Select AWS region"
        placeholder="Select an AWS region..."
        required={true}
        rule={requiredRegion}
      />
    </Validation>
  );
};

export const Azure = () => {
  const [selectedRegions, setSelectedRegions] = useState<
    readonly Option<AzureRegion>[]
  >([]);

  const [selectedRegion, setSelectedRegion] =
    useState<Option<AzureRegion> | null>(null);

  return (
    <Validation>
      <RegionSelect
        isMulti={true}
        value={selectedRegions}
        onChange={setSelectedRegions}
        options={azureRegionOptions}
        label="Select Azure regions"
        placeholder="Select Azure regions..."
        required={true}
        rule={requiredAtLeastOneRegion}
      />

      <RegionSelect
        isMulti={false}
        value={selectedRegion}
        onChange={setSelectedRegion}
        options={azureRegionOptions}
        label="Select AWS region"
        placeholder="Select an Azure region..."
        required={true}
        rule={requiredRegion}
      />
    </Validation>
  );
};
