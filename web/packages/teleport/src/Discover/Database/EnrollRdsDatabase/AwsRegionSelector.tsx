/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';
import { Box, Text, Flex, ButtonSecondary, LabelInput } from 'design';
import Select, { Option } from 'shared/components/Select';
import { Refresh as RefreshIcon } from 'design/Icon';

import { awsRegionMap, Regions } from 'teleport/services/integrations';

export function AwsRegionSelector({
  onFetch,
  onRefresh,
  disableSelector,
  clear,
}: {
  onFetch(region: Regions): void;
  onRefresh(): void;
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
      <Text mt={4}>
        Select the AWS Region you would like to see databases for:
      </Text>
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
        <ButtonSecondary
          onClick={onRefresh}
          mt={1}
          title="Refresh database table"
          height="40px"
          width="30px"
          css={`
            &:disabled {
              opacity: 0.35;
              pointer-events: none;
            }
          `}
          disabled={disableSelector || !selectedRegion}
        >
          <RefreshIcon fontSize={3} />
        </ButtonSecondary>
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
