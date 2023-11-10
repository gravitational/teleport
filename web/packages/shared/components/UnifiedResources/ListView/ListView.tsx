/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { Flex, Box, Indicator } from 'design';

import { ResourceViewProps } from '../types';

import { ResourceListItem } from './ResourceListItem';

export function ListView({
  mappedResources,
  onLabelClick,
  pinnedResources,
  selectedResources,
  onSelectResource,
  onPinResource,
  pinningSupport,
  isProcessing,
}: ResourceViewProps) {
  return (
    <Flex className="ListContainer">
      {mappedResources.map(({ item, key }) => (
        <ResourceListItem
          key={key}
          name={item.name}
          ActionButton={item.ActionButton}
          primaryIconName={item.primaryIconName}
          onLabelClick={onLabelClick}
          SecondaryIcon={item.SecondaryIcon}
          listViewProps={item.listViewProps}
          labels={item.labels}
          pinned={pinnedResources.includes(key)}
          pinningSupport={pinningSupport}
          selected={selectedResources.includes(key)}
          selectResource={() => onSelectResource(key)}
          pinResource={() => onPinResource(key)}
        />
      ))}
      {/* TODO (rudream): Add skeleton loader */}
      {isProcessing && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
    </Flex>
  );
}
