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
        <Text color="text.placeholder">No Connections</Text>
      )}
    </Box>
  );
}
