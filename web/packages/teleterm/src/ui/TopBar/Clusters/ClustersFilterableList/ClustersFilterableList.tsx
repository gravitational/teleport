import React from 'react';

import { Box, Text } from 'design';

import { FilterableList } from 'teleterm/ui/components/FilterableList';
import { useKeyboardArrowsNavigationStateUpdate } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { Cluster } from 'teleterm/services/tshd/types';

import { ClusterItem } from './ClusterItem';

interface ClustersFilterableListProps {
  items: Cluster[];
  selectedItem: Cluster;

  onSelectItem(clusterUri: string): void;
}

export function ClustersFilterableList(props: ClustersFilterableListProps) {
  const { setActiveIndex } = useKeyboardArrowsNavigationStateUpdate();
  return (
    <Box width="260px">
      {props.items.length ? (
        <FilterableList<Cluster>
          items={props.items}
          filterBy="name"
          onFilterChange={value =>
            value.length ? setActiveIndex(0) : setActiveIndex(-1)
          }
          placeholder="Search Leaf Cluster"
          Node={({ item, index }) => (
            <ClusterItem
              item={item}
              index={index}
              onSelect={() => props.onSelectItem(item.uri)}
              isSelected={props.selectedItem === item}
            />
          )}
        />
      ) : (
        <Text color="text.placeholder">No Clusters</Text>
      )}
    </Box>
  );
}
