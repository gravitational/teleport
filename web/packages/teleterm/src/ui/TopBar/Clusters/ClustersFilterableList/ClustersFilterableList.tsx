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
        <Text color="text.muted">No Clusters</Text>
      )}
    </Box>
  );
}
