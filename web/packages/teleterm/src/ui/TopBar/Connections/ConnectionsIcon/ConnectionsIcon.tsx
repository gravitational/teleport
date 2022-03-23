import React, { forwardRef } from 'react';
import { Cluster } from 'design/Icon';
import styled from 'styled-components';
import { Button } from 'design';
import { ConnectionsIconStatusIndicator } from './ConnectionsIconStatusIndicator';

interface ConnectionsIconProps {
  isAnyConnectionActive: boolean;

  onClick(): void;
}

export const ConnectionsIcon = forwardRef<HTMLDivElement, ConnectionsIconProps>(
  (props, ref) => {
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
          title="Connections"
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
  background: ${props => props.theme.colors.primary.lighter};
  padding: 9px;
  width: 30px;
  height: 30px;
`;
