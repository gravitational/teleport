import React from 'react';
import { FilterableList } from 'teleterm/ui/components/FilterableList';
import { ClusterItem } from './ClusterItem';
import { Box } from 'design';
import { useKeyboardArrowsNavigationStateUpdate } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { Cluster } from 'teleterm/services/tshd/types';

interface ClustersFilterableListProps {
  items: Cluster[];
  selectedItem: Cluster;

  onSelectItem(clusterUri: string): void;
}

export function ClustersFilterableList(props: ClustersFilterableListProps) {
  const { setActiveIndex } = useKeyboardArrowsNavigationStateUpdate();
  return (
    <Box width="260px">
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
    </Box>
  );
}
