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
  background: ${props => props.theme.colors.levels.surface};
`;
