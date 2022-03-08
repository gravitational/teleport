import React, { useCallback, useMemo, useRef, useState } from 'react';
import Popover from 'design/Popover';
import styled from 'styled-components';
import { Box } from 'design';
import { useConnections } from './useConnections';
import { ConnectionsIcon } from './ConnectionsIcon/ConnectionsIcon';
import { ConnectionsFilterableList } from './ConnectionsFilterableList/ConnectionsFilterableList';
import { useKeyboardShortcuts } from 'teleterm/ui/services/keyboardShortcuts';

export function Connections() {
  const iconRef = useRef();
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const connections = useConnections();

  const togglePopover = useCallback(() => {
    setIsPopoverOpened(wasOpened => !wasOpened);
  }, [setIsPopoverOpened]);

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
          <ConnectionsFilterableList
            items={connections.items}
            onActivateItem={activateItem}
            onRemoveItem={connections.removeItem}
            onDisconnectItem={connections.disconnectItem}
          />
        </Container>
      </Popover>
    </>
  );
}

const Container = styled(Box)`
  background: ${props => props.theme.colors.primary.dark};
`;
