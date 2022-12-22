import React, { forwardRef } from 'react';
import { Cluster } from 'design/Icon';
import styled from 'styled-components';
import { Button } from 'design';

import { useKeyboardShortcutFormatters } from 'teleterm/ui/services/keyboardShortcuts';

import { ConnectionsIconStatusIndicator } from './ConnectionsIconStatusIndicator';

interface ConnectionsIconProps {
  isAnyConnectionActive: boolean;

  onClick(): void;
}

export const ConnectionsIcon = forwardRef<HTMLDivElement, ConnectionsIconProps>(
  (props, ref) => {
    const { getLabelWithShortcut } = useKeyboardShortcutFormatters();
    return (
      <Container ref={ref}>
        <ConnectionsIconStatusIndicator
          connected={props.isAnyConnectionActive}
        />
        <StyledButton
          onClick={props.onClick}
          kind="secondary"
          size="small"
          m="auto"
          title={getLabelWithShortcut('Open Connections', 'toggle-connections')}
        >
          <Cluster fontSize={16} />
        </StyledButton>
      </Container>
    );
  }
);

const Container = styled.div`
  position: relative;
  display: inline-block;
`;

const StyledButton = styled(Button)`
  background: ${props => props.theme.colors.primary.light};
  padding: 9px;
  width: 30px;
  height: 30px;
`;
