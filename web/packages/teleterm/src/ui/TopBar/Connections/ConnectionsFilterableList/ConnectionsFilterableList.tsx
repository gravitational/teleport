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

import { Box, Text } from 'design';

import { FilterableList } from 'teleterm/ui/components/FilterableList';
import { ExtendedTrackedConnection } from 'teleterm/ui/services/connectionTracker';

import { useKeyboardArrowsNavigationStateUpdate } from 'teleterm/ui/components/KeyboardArrowsNavigation';

import { ConnectionItem } from './ConnectionItem';

interface ConnectionsFilterableListProps {
  items: ExtendedTrackedConnection[];

  onActivateItem(id: string): void;

  onRemoveItem(id: string): void;

  onDisconnectItem(id: string): void;
}

export function ConnectionsFilterableList(
  props: ConnectionsFilterableListProps
) {
  const { setActiveIndex } = useKeyboardArrowsNavigationStateUpdate();

  return (
    <Box width="300px">
      {props.items.length ? (
        <FilterableList<ExtendedTrackedConnection>
          items={props.items}
          filterBy="title"
          placeholder="Search Connections"
          onFilterChange={value =>
            value.length ? setActiveIndex(0) : setActiveIndex(-1)
          }
          Node={({ item, index }) => (
            <ConnectionItem
              item={item}
              index={index}
              onActivate={() => props.onActivateItem(item.id)}
              onRemove={() => props.onRemoveItem(item.id)}
              onDisconnect={() => props.onDisconnectItem(item.id)}
            />
          )}
        />
      ) : (
        <Text color="text.muted">No Connections</Text>
      )}
    </Box>
  );
}
