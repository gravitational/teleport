import React from 'react';
import { FilterableList } from 'teleterm/ui/components/FilterableList';
import { TrackedConnection } from 'teleterm/ui/services/connectionTracker';
import { ConnectionItem } from './ConnectionItem';
import { Box } from 'design';
import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';

interface ConnectionsFilterableListProps {
  items: TrackedConnection[];

  onActivateItem(id: string): void;

  onRemoveItem(id: string): void;

  onDisconnectItem(id: string): void;
}

export function ConnectionsFilterableList(
  props: ConnectionsFilterableListProps
) {
  return (
    <Box width="200px">
      <KeyboardArrowsNavigation>
        <FilterableList<TrackedConnection>
          items={props.items}
          filterBy="title"
          placeholder="Search Connections"
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
      </KeyboardArrowsNavigation>
    </Box>
  );
}
