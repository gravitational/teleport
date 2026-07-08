/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { forwardRef } from 'react';
import styled from 'styled-components';

import { ButtonSecondary } from 'design';
import { Broadcast } from 'design/Icon';

import {
  ConnectionsIconStatusIndicator,
  Status,
} from 'teleterm/ui/TopBar/Connections/ConnectionsIcon/ConnectionsIconStatusIndicator';

export const VnetIcon = forwardRef<
  HTMLDivElement,
  {
    status: Status;
    onClick(): void;
  }
>((props, ref) => {
  return (
    <Container ref={ref}>
      <ConnectionsIconStatusIndicator status={props.status} />
      <StyledButton
        onClick={props.onClick}
        size="small"
        m="auto"
        title="Open VNet"
        data-testid="vnet-icon"
      >
        <Broadcast size="medium" />
      </StyledButton>
    </Container>
  );
});

const Container = styled.div`
  position: relative;
  display: inline-block;
`;

const StyledButton = styled(ButtonSecondary)`
  padding: 0;
  width: ${props => props.theme.space[5]}px;
  height: ${props => props.theme.space[5]}px;
`;
