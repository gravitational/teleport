/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, { useState } from 'react';

import { Box, ButtonSecondary, Flex, LabelInput } from 'design';
import { Refresh as RefreshIcon } from 'design/Icon';
import Select, { Option } from 'shared/components/Select';

import { awsRegionMap, Regions } from 'teleport/services/integrations';

export function AwsRegionSelector({
  onFetch,
  onRefresh,
  disableSelector,
  clear,
}: {
  onFetch(region: Regions): void;
  onRefresh?(): void;
  disableSelector: boolean;
  clear(): void;
}) {
  const [selectedRegion, setSelectedRegion] = useState<RegionOption>();

  function handleRegionSelect(option: RegionOption) {
    clear();
    setSelectedRegion(option);
    onFetch(option.value);
  }

  return (
    <Box>
      <Flex alignItems="center" gap={3} mt={2} mb={3}>
        <Box width="320px" mb={4}>
          <LabelInput htmlFor={'select'}>AWS Region</LabelInput>
          <Select
            inputId="select"
            isSearchable
            value={selectedRegion}
            onChange={handleRegionSelect}
            options={options}
            placeholder="Select a region"
            autoFocus
            isDisabled={disableSelector}
          />
        </Box>
        {onRefresh && (
          <ButtonSecondary
            onClick={onRefresh}
            mt={1}
            title="Refresh"
            height="40px"
            width="40px"
            p={0}
            disabled={disableSelector || !selectedRegion}
          >
            <RefreshIcon size="medium" />
          </ButtonSecondary>
        )}
      </Flex>
    </Box>
  );
}

type RegionOption = Option<Regions, React.ReactElement>;

const options: RegionOption[] = Object.keys(awsRegionMap).map(region => ({
  value: region as Regions,
  label: (
    <Flex justifyContent="space-between">
      <div>{awsRegionMap[region]}&nbsp;&nbsp;</div>
      <div>{region}</div>
    </Flex>
  ),
}));
