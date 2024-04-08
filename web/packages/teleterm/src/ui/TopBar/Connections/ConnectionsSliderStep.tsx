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

import { useEffect } from 'react';
import { Box } from 'design';
import { StepComponentProps } from 'design/StepSlider';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';

import { ConnectionsFilterableList } from './ConnectionsFilterableList/ConnectionsFilterableList';
import { useConnections } from './useConnections';

export const ConnectionsSliderStep = (
  props: StepComponentProps & { closeConnectionList: () => void }
) => {
  const { connectionTracker } = useAppContext();
  const connections = useConnections();
  const { updateSorting } = connections;

  const activateItem = (id: string) => {
    props.closeConnectionList();
    connectionTracker.activateItem(id, { origin: 'connection_list' });
  };

  useEffect(() => {
    updateSorting();
    // Sorting needs to be updated only once when the component gets rendered. This is so that when
    // new connections are added while the list is open, the sorting is _not_ updated and new items
    // end up at the top of the list.
    //
    // This also keeps the list stable when you e.g. disconnect the first connection in the list.
    // Instead of the item jumping around, it stays as the first until the user closes and then
    // opens the list again.
  }, []);

  return (
    <Box p={2} ref={props.refCallback}>
      <KeyboardArrowsNavigation>
        <ConnectionsFilterableList
          items={connections.items}
          onActivateItem={activateItem}
          onRemoveItem={connections.removeItem}
          onDisconnectItem={connections.disconnectItem}
          slideToVnet={props.next}
        />
      </KeyboardArrowsNavigation>
    </Box>
  );
};
