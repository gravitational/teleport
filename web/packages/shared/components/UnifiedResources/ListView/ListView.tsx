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

import {
  generateUnifiedResourceKey,
  getResourcePinningSupport,
} from '../UnifiedResources';

import { ResourceViewProps } from '../types';

import { mapResourceToItem } from '../shared';

import { ResourceListItem } from './ResourceListItem';

export function ListView({
  resources,
  onLabelClick,
  pinnedResources,
  pinning,
  updatePinnedResourcesAttempt,
  selectedResources,
  handleSelectResources,
  handlePinResource,
  isProcessing,
}: ResourceViewProps) {
  return (
    <Flex className="ListContainer">
      {resources
        .map(unifiedResource => ({
          item: mapResourceToItem(unifiedResource),
          key: generateUnifiedResourceKey(unifiedResource.resource),
        }))
        .map(({ item, key }) => (
          <ResourceListItem
            key={key}
            name={item.name}
            ActionButton={item.ActionButton}
            primaryIconName={item.primaryIconName}
            onLabelClick={onLabelClick}
            SecondaryIcon={item.SecondaryIcon}
            description={item.description}
            addr={item.addr}
            type={item.type}
            labels={item.labels}
            pinned={pinnedResources.includes(key)}
            pinningSupport={getResourcePinningSupport(
              pinning.kind,
              updatePinnedResourcesAttempt
            )}
            selected={selectedResources.includes(key)}
            selectResource={() => handleSelectResources(key)}
            pinResource={() => handlePinResource(key)}
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
