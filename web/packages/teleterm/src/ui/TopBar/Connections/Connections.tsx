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

import React, { useCallback, useMemo, useRef, useState } from 'react';
import Popover from 'design/Popover';
import styled from 'styled-components';
import { Box } from 'design';

import { useKeyboardShortcuts } from 'teleterm/ui/services/keyboardShortcuts';

import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';

import { useConnections } from './useConnections';
import { ConnectionsIcon } from './ConnectionsIcon/ConnectionsIcon';
import { ConnectionsFilterableList } from './ConnectionsFilterableList/ConnectionsFilterableList';

export function Connections() {
  const iconRef = useRef();
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const connections = useConnections();

  const togglePopover = useCallback(() => {
    setIsPopoverOpened(wasOpened => {
      const isOpened = !wasOpened;
      if (isOpened) {
        connections.updateSorting();
      }
      return isOpened;
    });
  }, [setIsPopoverOpened, connections.updateSorting]);

  useKeyboardShortcuts(
    useMemo(
      () => ({
        openConnections: togglePopover,
      }),
      [togglePopover]
    )
  );

  function activateItem(id: string): void {
    setIsPopoverOpened(false);
    connections.activateItem(id);
  }

  return (
    <>
      <ConnectionsIcon
        isAnyConnectionActive={connections.isAnyConnectionActive}
        onClick={togglePopover}
        ref={iconRef}
      />
      <Popover
        open={isPopoverOpened}
        anchorEl={iconRef.current}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        onClose={() => setIsPopoverOpened(false)}
      >
        <Container p="12px">
          <KeyboardArrowsNavigation>
            <ConnectionsFilterableList
              items={connections.items}
              onActivateItem={activateItem}
              onRemoveItem={connections.removeItem}
              onDisconnectItem={connections.disconnectItem}
            />
          </KeyboardArrowsNavigation>
        </Container>
      </Popover>
    </>
  );
}

const Container = styled(Box)`
  background: ${props => props.theme.colors.levels.elevated};
`;
