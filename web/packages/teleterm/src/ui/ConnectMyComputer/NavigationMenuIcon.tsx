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
import styled from 'styled-components';
import { Laptop } from 'design/Icon';
import { Box, Button } from 'design';

import { AgentState } from './connectMyComputerContext';

interface NavigationMenuIconProps {
  onClick(): void;
  agentState: AgentState;
}

export const NavigationMenuIcon = forwardRef<
  HTMLDivElement,
  NavigationMenuIconProps
>((props, ref) => {
  return (
    <StyledButton
      setRef={ref}
      onClick={props.onClick}
      kind="secondary"
      size="small"
      title="Open Connect My Computer"
    >
      <Laptop size="medium" />
      {getStateIndicator(props.agentState)}
    </StyledButton>
  );
});

const StyledButton = styled(Button)`
  position: relative;
  background: ${props => props.theme.colors.spotBackground[0]};
  padding: 0;
  width: ${props => props.theme.space[5]}px;
  height: ${props => props.theme.space[5]}px;
`;

function getStateIndicator(agentState: AgentState): JSX.Element {
  switch (agentState.status) {
    case 'starting':
    case 'stopping': {
      return (
        <StyledStatus
          bg="success"
          css={`
            @keyframes blink {
              0% {
                opacity: 0;
              }
              50% {
                opacity: 100%;
              }
              100% {
                opacity: 0;
              }
            }

            animation: blink 1.4s ease-in-out infinite;
          `}
        />
      );
    }
    case 'running': {
      return <StyledStatus bg="success" />;
    }
    case 'error': {
      return <StyledStatus bg="error.main" />;
    }
    case 'exited': {
      if (!agentState.exitedSuccessfully) {
        return <StyledStatus bg="error.main" />;
      }
    }
  }
}

const StyledStatus = styled(Box)`
  position: absolute;
  top: -4px;
  right: -4px;
  z-index: 1;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
`;
