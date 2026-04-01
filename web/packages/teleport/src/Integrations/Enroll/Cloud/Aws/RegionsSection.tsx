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

import { useMemo } from 'react';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { FieldRadio } from 'design/FieldRadio';
import { Option } from 'shared/components/Select';
import { Rule } from 'shared/components/Validation/rules';

import { Regions as AwsRegion } from 'teleport/services/integrations';

import { CircleNumber, RegionOrWildcard } from '../Shared';
import { RegionSelect } from '../Shared/RegionSelect';
import { awsRegionOptions } from './regions';

type RegionsSectionProps = {
  regions: RegionOrWildcard<AwsRegion>[];
  onChange: (regions: RegionOrWildcard<AwsRegion>[]) => void;
};

const isWildcard = (regions: RegionOrWildcard<AwsRegion>[]) =>
  regions.some(region => region === '*');

export function RegionsSection({ regions, onChange }: RegionsSectionProps) {
  const selectedOptions: Option<AwsRegion>[] = useMemo(() => {
    if (isWildcard(regions)) {
      return [];
    }

    const allOptions = awsRegionOptions.flatMap(group => group.options);
    return allOptions.filter(opt => regions.includes(opt.value));
  }, [regions]);

  const requiredAtLeastOneRegion: Rule<Option<AwsRegion>[]> =
    (options: Option<AwsRegion>[]) => () => {
      if (!options || options.length === 0) {
        return {
          valid: false,
          message: 'At least one region must be selected',
        };
      }
      return { valid: true };
    };

  return (
    <>
      <Flex alignItems="center" fontSize={4} fontWeight="medium" mb={1}>
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
            <Flex alignItems="center">
              <RadioLabel selected={false}>All regions</RadioLabel>
            </Flex>
          }
          size="small"
          checked={isWildcard(regions)}
          onChange={() => onChange(['*'])}
          mb={1}
        />
        <FieldRadio
          name="regions"
          label={
            <Flex alignItems="center">
              <RadioLabel selected={false}>Select specific regions</RadioLabel>
            </Flex>
          }
          size="small"
          checked={!isWildcard(regions)}
          onChange={() => onChange([])}
          mb={1}
        />

        {!isWildcard(regions) && (
          <Box mt={3} width={400}>
            <RegionSelect
              isMulti={true}
              options={awsRegionOptions}
              value={selectedOptions}
              onChange={options => onChange(options.map(opt => opt.value))}
              isDisabled={false}
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
