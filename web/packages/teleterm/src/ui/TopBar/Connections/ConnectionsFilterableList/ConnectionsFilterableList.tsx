import React from 'react';
import { FilterableList } from 'teleterm/ui/components/FilterableList';
import { TrackedConnection } from 'teleterm/ui/services/connectionTracker';
import { ConnectionItem } from './ConnectionItem';
import { Box } from 'design';
import { useKeyboardArrowsNavigationStateUpdate } from 'teleterm/ui/components/KeyboardArrowsNavigation';

interface ConnectionsFilterableListProps {
  items: TrackedConnection[];

  onActivateItem(id: string): void;

  onRemoveItem(id: string): void;

  onDisconnectItem(id: string): void;
}

export function ConnectionsFilterableList(
  props: ConnectionsFilterableListProps
) {
  const { setActiveIndex } = useKeyboardArrowsNavigationStateUpdate();

  return (
    <Box width="200px">
      <FilterableList<TrackedConnection>
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
    </Box>
  );
}
