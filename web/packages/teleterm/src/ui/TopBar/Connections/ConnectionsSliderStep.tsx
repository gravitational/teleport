/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useState } from 'react';
import { Box } from 'design';
import { StepComponentProps } from 'design/StepSlider';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';

import { ConnectionsFilterableList } from './ConnectionsFilterableList/ConnectionsFilterableList';
import { useConnectionsContext } from './connectionsContext';

export const ConnectionsSliderStep = (props: StepComponentProps) => {
  const { connectionTracker } = useAppContext();
  connectionTracker.useState();
  const { close: closeConnectionList } = useConnectionsContext();

  const items = connectionTracker.getConnections();
  const [sortedIds, setSortedIds] = useState<string[]>(null);

  // Sorting needs to be updated only once when the component gets first rendered. This is so that
  // when new connections are added while the list is open, the sorting is _not_ updated and new
  // items end up at the top of the list.
  //
  // This also keeps the list stable when you e.g. disconnect the first connection in the list.
  // Instead of the item jumping around, it stays as the top until the user closes and then opens
  // the list again.
  if (sortedIds === null) {
    const sorted = items
      .slice()
      // New connections are pushed to the list in `connectionTracker`, so we have to reverse them
      // to get the newest items on the top
      .reverse()
      // Connected items first.
      .sort((a, b) => (a.connected === b.connected ? 0 : a.connected ? -1 : 1))
      .map(a => a.id);
    setSortedIds(sorted);
    return null;
  }

  const sortedItems =
    // It is possible that new connections are added when the menu is open.
    // They will have -1 index and appear on the top.
    // Items are sorted by insertion order, meaning that if I add A then B,
    // then close both, open A and close it, it's going to appear after B
    // even though it was used more recently than B.
    items
      .slice()
      .sort((a, b) => sortedIds.indexOf(a.id) - sortedIds.indexOf(b.id));

  const removeItem = connectionTracker.removeItem.bind(connectionTracker);
  const disconnectItem =
    connectionTracker.disconnectItem.bind(connectionTracker);
  const activateItem = (id: string) => {
    closeConnectionList();
    connectionTracker.activateItem(id, { origin: 'connection_list' });
  };

  return (
    <Box p={2} ref={props.refCallback}>
      <KeyboardArrowsNavigation>
        <ConnectionsFilterableList
          items={sortedItems}
          activateItem={activateItem}
          removeItem={removeItem}
          disconnectItem={disconnectItem}
          slideToVnet={props.next}
        />
      </KeyboardArrowsNavigation>
    </Box>
  );
};
