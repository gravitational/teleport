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
import { useAppContext } from 'teleterm/ui/appContextProvider';

import { ConnectionItem } from './ConnectionItem';

export function ConnectionsFilterableList(props: {
  items: ExtendedTrackedConnection[];
  onActivateItem(id: string): void;
  onRemoveItem(id: string): void;
  onDisconnectItem(id: string): void;
}) {
  const { setActiveIndex } = useKeyboardArrowsNavigationStateUpdate();
  const { clustersService } = useAppContext();
  const clustersInConnections = new Set(props.items.map(i => i.clusterName));
  // showClusterNames is based on two values, as there are two cases we need to account for:
  //
  // 1. If there's only a single cluster a user has access to, they don't care about its name.
  // However, the moment there's an extra leaf cluster or just another profile, the user might want
  // to know the name of a cluster for the given connection, even if the connection list currently
  // shows connections only from a single cluster.
  //
  // 2. The connection list might include a connection to a leaf cluster resource even after that
  // leaf is no longer available and there's only a single cluster in clustersService. As such, we
  // have to look at the number of clusters in connections as well.
  const showClusterName =
    clustersService.getClustersCount() > 1 || clustersInConnections.size > 1;

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
              showClusterName={showClusterName}
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
