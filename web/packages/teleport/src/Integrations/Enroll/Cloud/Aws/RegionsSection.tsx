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

import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { FieldRadio } from 'design/FieldRadio';
import { Rule } from 'shared/components/Validation/rules';

import { Regions as AwsRegion } from 'teleport/services/integrations';

import { RegionMultiSelector } from '../RegionMultiSelector';
import { CircleNumber } from './EnrollAws';
import { awsRegionGroups } from './regions';
import { WildcardRegion } from './types';

type RegionsOrWildcard = WildcardRegion | AwsRegion[];

type RegionsSectionProps = {
  regions: RegionsOrWildcard;
  onChange: (regions: RegionsOrWildcard) => void;
};

const isWildcard = (regions: RegionsOrWildcard): regions is WildcardRegion =>
  regions.length === 1 && regions[0] === '*';

export function RegionsSection({ regions, onChange }: RegionsSectionProps) {
  const requiredAtLeastOneRegion: Rule<RegionsOrWildcard> =
    (regions: RegionsOrWildcard) => () => {
      if (isWildcard(regions)) {
        return { valid: true };
      }

      if (!regions || regions.length === 0) {
        return {
          valid: false,
          message: 'At least one region must be selected',
        };
      }
      return { valid: true };
    };

  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium">
        <CircleNumber>4</CircleNumber>
        Regions
      </Flex>
      <Text mb={3} ml={4}>
        Select the AWS regions where your resources are located.
      </Text>
      <Box ml={4}>
        <FieldRadio
          name="regions"
          label={
            <Flex alignItems="center" gap={2}>
              <RadioLabel selected={isWildcard(regions)}>
                All Regions
              </RadioLabel>
            </Flex>
          }
          size="small"
          checked={isWildcard(regions)}
          onChange={() => onChange(['*'])}
        />
        <FieldRadio
          name="regions"
          label={
            <Flex alignItems="center" gap={2}>
              <RadioLabel selected={!isWildcard}>
                Select specific Regions
              </RadioLabel>
            </Flex>
          }
          size="small"
          checked={!isWildcard(regions)}
          onChange={() => onChange([])}
          mb={0}
        />

        {!isWildcard(regions) && (
          <Box mt={3}>
            <RegionMultiSelector
              regionGroups={awsRegionGroups}
              selectedRegions={regions}
              onChange={regions => onChange(regions)}
              disabled={false}
              required={true}
              rule={requiredAtLeastOneRegion}
            />
          </Box>
        )}
      </Box>
    </>
  );
}

const RadioLabel = styled(Flex)<{ selected: boolean }>`
  font-weight: ${props => (props.selected ? '600' : 'inherit')};
`;
