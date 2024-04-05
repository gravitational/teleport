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

import { Text } from 'design';

import { FilterableList } from 'teleterm/ui/components/FilterableList';
import { ExtendedTrackedConnection } from 'teleterm/ui/services/connectionTracker';
import { useKeyboardArrowsNavigationStateUpdate } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { VnetConnectionItem, useVnetContext } from 'teleterm/ui/Vnet';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import { ConnectionItem } from './ConnectionItem';

export function ConnectionsFilterableList(props: {
  items: ExtendedTrackedConnection[];
  onActivateItem(id: string): void;
  onRemoveItem(id: string): void;
  onDisconnectItem(id: string): void;
  slideToVnet(): void;
}) {
  const { setActiveIndex } = useKeyboardArrowsNavigationStateUpdate();
  const { isSupported: isVnetSupported } = useVnetContext();
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

  if (!isVnetSupported && props.items.length === 0) {
    return <Text color="text.muted">No Connections</Text>;
  } // With VNet being supported, there's always at least one item to show – the VNet item.

  return (
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
    >
      {/*
        TODO(ravicious): Change the type of FilterableList above to something like
        FilterableList<ExtendedTrackedConnection | Vnet> and render a different component in Node
        depending on the item type. This way VNet will have tighter integration with the connection
        list, i.e. be searchable and selectable through keyboard.

        We don't want to put VNet into ExtendedTrackedConnection because these are two fundamentally
        different things.
      */}
      {isVnetSupported && (
        <VnetConnectionItem
          onClick={props.slideToVnet}
          title="Open VNet panel"
        />
      )}
    </FilterableList>
  );
}
