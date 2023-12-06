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
