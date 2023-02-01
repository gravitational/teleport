/**
 * Copyright 2022 Gravitational, Inc.
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
import { Cloud } from 'design/Icon';
import { Text, Box, Flex } from 'design';
import styled from 'styled-components';

import SlideTabs from 'design/SlideTabs';

import type { ResourceType, ResourceLocation } from '../resource-lists';

export function SelectDBDeploymentType({
  selectedType,
  setSelectedType,
  resourceTypes,
}: SelectDBDeploymentTypeProps) {
  type FilterType = 'All' | ResourceLocation;
  const filterTabs: FilterType[] = ['All', 'AWS', 'Self-Hosted'];
  const [filter, setFilter] = useState<FilterType>('All');
  return (
    <Box mt={6}>
      <Flex alignItems="center" justifyContent="space-between">
        <Text mb={2}>Select Deployment Type</Text>
        <Box width="379px">
          <SlideTabs
            appearance="round"
            size="medium"
            tabs={filterTabs}
            onChange={index => setFilter(filterTabs[index])}
          />
        </Box>
      </Flex>
      <Flex
        flexWrap="wrap"
        mt={4}
        justifyContent="space-between"
        gap="12px 12px"
        rowGap="15px"
      >
        {resourceTypes
          .filter(resource => filter === 'All' || resource.type === filter)
          .map(resource => (
            <ResourceTypeOption
              onClick={() => setSelectedType(resource.key)}
              key={resource.key}
              selected={selectedType === resource.key}
            >
              <Flex justifyContent="space-between" mb={2}>
                <Cloud />
                <Tag>popular</Tag>
              </Flex>
              {resource.name}
            </ResourceTypeOption>
          ))}
      </Flex>
    </Box>
  );
}

type SelectDBDeploymentTypeProps = {
  selectedType: string;
  setSelectedType: (string) => void;
  resourceTypes: ResourceType[];
};

const ResourceTypeOption = styled.div`
  background: rgba(255, 255, 255, 0.05);
  border: ${props =>
    !props.selected
      ? '2px solid rgba(255, 255, 255, 0)'
      : '2px solid rgba(255, 255, 255, 0.1);'};
  border-radius: 8px;
  box-sizing: border-box;
  cursor: pointer;
  height: 72px;
  padding: 12px;
  width: 242px;

  &:hover {
    border: 2px solid rgba(255, 255, 255, 0.1);
  }
`;

const Tag = styled.div`
  align-items: center;
  background-color: #512fc9;
  border-radius: 33px;
  box-sizing: border-box;
  font-size: 10px;
  height: 15px;
  line-height: 11px;
  padding: 2px 10px;
  max-width: 57px;
`;

export type SelectResourceProps = {
  onSelect: (string) => void;
};
