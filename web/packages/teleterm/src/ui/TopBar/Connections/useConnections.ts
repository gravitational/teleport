/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
