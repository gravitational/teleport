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

import { Box, Text } from 'design';

import { Cluster } from 'teleterm/services/tshd/types';
import { FilterableList } from 'teleterm/ui/components/FilterableList';
import { useKeyboardArrowsNavigationStateUpdate } from 'teleterm/ui/components/KeyboardArrowsNavigation';

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
          placeholder="Search leaf clusters"
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
        <Text color="text.muted">No Clusters</Text>
      )}
    </Box>
  );
}
