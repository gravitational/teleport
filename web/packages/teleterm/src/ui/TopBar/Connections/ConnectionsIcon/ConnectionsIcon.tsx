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
    const { getLabelWithAccelerator } = useKeyboardShortcutFormatters();
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
          title={getLabelWithAccelerator('Open Connections', 'openConnections')}
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
  background: ${props => props.theme.colors.spotBackground[0]};
  padding: 0;
  width: ${props => props.theme.space[5]}px;
  height: ${props => props.theme.space[5]}px;
`;
