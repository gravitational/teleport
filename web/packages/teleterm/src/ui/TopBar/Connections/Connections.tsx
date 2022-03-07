import React, { useRef, useState } from 'react';
import Popover from 'design/Popover';
import styled from 'styled-components';
import { Box } from 'design';
import { useConnections } from './useConnections';
import { ConnectionsIcon } from './ConnectionsIcon/ConnectionsIcon';
import { ConnectionsFilterableList } from './ConnectionsFilterableList/ConnectionsFilterableList';

export function Connections() {
  const iconRef = useRef();
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const connections = useConnections();

  function togglePopover(): void {
    setIsPopoverOpened(wasOpened => !wasOpened);
  }

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
