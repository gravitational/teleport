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

import { useCallback, useState } from 'react';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ExtendedTrackedConnection } from 'teleterm/ui/services/connectionTracker';

export function useConnections() {
  const { connectionTracker } = useAppContext();

  connectionTracker.useState();

  const items = connectionTracker.getConnections();
  const [sortedIds, setSortedIds] = useState<string[]>([]);

  const getSortedItems = () => {
    const findIndexInSorted = (item: ExtendedTrackedConnection) =>
      sortedIds.indexOf(item.id);
    // It is possible that new connections are added when the menu is open
    // they will have -1 index and appear on the top.
    // Items are sorted by insertion order, meaning that if I add A then B
    // then close both, open A and close it, it's going to appear after B
    // even though it was used more recently than B
    return [...items].sort(
      (a, b) => findIndexInSorted(a) - findIndexInSorted(b)
    );
  };

  const serializedItems = items.map(i => `${i.id}${i.connected}`).join(',');
  const updateSorting = useCallback(() => {
    const sorted = [...items]
      // new connections are pushed to the list in `connectionTracker`,
      // so we have to reverse them to get the newest items on the top
      .reverse()
      // connected first
      .sort((a, b) => (a.connected === b.connected ? 0 : a.connected ? -1 : 1))
      .map(a => a.id);

    setSortedIds(sorted);
  }, [setSortedIds, serializedItems]);

  return {
    isAnyConnectionActive: items.some(c => c.connected),
    removeItem: (id: string) => connectionTracker.removeItem(id),
    activateItem: (id: string) =>
      connectionTracker.activateItem(id, { origin: 'connection_list' }),
    disconnectItem: (id: string) => connectionTracker.disconnectItem(id),
    updateSorting,
    items: getSortedItems(),
  };
}
