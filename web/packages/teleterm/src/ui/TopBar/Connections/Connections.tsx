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
        'toggle-connections': togglePopover,
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
  background: ${props => props.theme.colors.primary.light};
`;
