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

import { ConnectionItem } from './ConnectionItem';

export function ConnectionsFilterableList(props: {
  items: ExtendedTrackedConnection[];
  activateItem(id: string): void;
  removeItem(id: string): void;
  disconnectItem(id: string): void;
  slideToVnet(): void;
}) {
  const { setActiveIndex } = useKeyboardArrowsNavigationStateUpdate();
  const { isSupported: isVnetSupported } = useVnetContext();

  if (!isVnetSupported && props.items.length === 0) {
    return <Text color="text.muted">No Connections</Text>;
  } // With VNet being supported, there's always at least one item to show – the VNet item.

  let items: Array<ExtendedTrackedConnection | VnetConnection> = props.items;

  if (isVnetSupported) {
    items = [{ kind: 'vnet', title: 'VNet' }, ...items];
  }

  return (
    <FilterableList<ExtendedTrackedConnection | VnetConnection>
      items={items}
      filterBy="title"
      placeholder="Search Connections"
      onFilterChange={value =>
        value.length ? setActiveIndex(0) : setActiveIndex(-1)
      }
      Node={({ item, index }) =>
        item.kind === 'vnet' ? (
          <VnetConnectionItem
            openVnetPanel={props.slideToVnet}
            title="Open VNet panel"
            index={index}
          />
        ) : (
          <ConnectionItem
            item={item}
            index={index}
            activate={() => props.activateItem(item.id)}
            remove={() => props.removeItem(item.id)}
            disconnect={() => props.disconnectItem(item.id)}
          />
        )
      }
    />
  );
}

type VnetConnection = { kind: 'vnet'; title: 'VNet' };
