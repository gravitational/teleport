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

import React, { forwardRef, useRef, useState } from 'react';
import styled, { css } from 'styled-components';
import { Box, Button, Indicator, Menu, MenuItem } from 'design';
import { Laptop, Warning } from 'design/Icon';

import { Attempt, AttemptStatus } from 'shared/hooks/useAsync';

import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { assertUnreachable } from 'teleterm/ui/utils';

import {
  CurrentAction,
  useConnectMyComputerContext,
} from './connectMyComputerContext';

import type { AgentCompatibility } from './CompatibilityPromise';

/**
 * IndicatorStatus combines a couple of different states into a single enum which dictates the
 * decorative look of NavigationMenu.
 */
type IndicatorStatus = AttemptStatus;

export function NavigationMenu() {
  const iconRef = useRef();
  const [isMenuOpened, setIsMenuOpened] = useState(false);
  const { documentsService, rootClusterUri } = useWorkspaceContext();
  const {
    isAgentConfiguredAttempt,
    agentCompatibility,
    currentAction,
    canUse,
  } = useConnectMyComputerContext();
  const indicatorStatus = getIndicatorStatus(
    currentAction,
    isAgentConfiguredAttempt,
    agentCompatibility
  );

  if (!canUse) {
    return null;
  }

  function toggleMenu() {
    setIsMenuOpened(wasOpened => !wasOpened);
  }

  function openDocument(): void {
    documentsService.openConnectMyComputerDocument({
      rootClusterUri,
    });
    setIsMenuOpened(false);
  }

  const setupMenuItem = (
    <MenuItem onClick={openDocument}>Connect My Computer</MenuItem>
  );
  const statusMenuItem = (
    <MenuItem onClick={openDocument}>
      {indicatorStatus === 'error' && (
        <Warning size="small" color="error.main" mr={1} />
      )}
      Manage agent
    </MenuItem>
  );

  return (
    <>
      <MenuIcon
        indicatorStatus={indicatorStatus}
        onClick={toggleMenu}
        ref={iconRef}
      />
      <Menu
        getContentAnchorEl={null}
        open={isMenuOpened}
        anchorEl={iconRef.current}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        transformOrigin={{ vertical: 'top', horizontal: 'left' }}
        onClose={() => setIsMenuOpened(false)}
        menuListCss={() =>
          css`
            display: flex;
            flex-direction: column;
          `
        }
      >
        {isAgentConfiguredAttempt.status === 'processing' && (
          <Indicator
            delay="none"
            css={`
              align-self: center;
            `}
          />
        )}
        {isAgentConfiguredAttempt.status === 'success' &&
          (!isAgentConfiguredAttempt.data ? setupMenuItem : statusMenuItem)}
        {isAgentConfiguredAttempt.status === 'error' && statusMenuItem}
      </Menu>
    </>
  );
}

function getIndicatorStatus(
  currentAction: CurrentAction,
  isAgentConfiguredAttempt: Attempt<boolean>,
  agentCompatibility: AgentCompatibility
): IndicatorStatus {
  if (isAgentConfiguredAttempt.status === 'error') {
    return 'error';
  }

  const isAgentConfigured =
    isAgentConfiguredAttempt.status === 'success' &&
    isAgentConfiguredAttempt.data;
  const isAgentCompatibilityUnkown = agentCompatibility === 'unknown';

  if (!isAgentConfigured || isAgentCompatibilityUnkown) {
    return '';
  }

  if (currentAction.kind === 'observe-process') {
    switch (currentAction.agentProcessState.status) {
      case 'not-started': {
        if (agentCompatibility === 'incompatible') {
          return 'error';
        }

        return '';
      }
      case 'error': {
        return 'error';
      }
      case 'exited': {
        return currentAction.agentProcessState.exitedSuccessfully
          ? ''
          : 'error';
      }
      case 'running': {
        return 'success';
      }
      default: {
        assertUnreachable(currentAction.agentProcessState);
      }
    }
  }
  if (
    currentAction.kind === 'remove' &&
    (currentAction.attempt.status === 'processing' ||
      currentAction.attempt.status === 'success')
  ) {
    return '';
  }
  return currentAction.attempt.status;
}

interface MenuIconProps {
  onClick(): void;
  indicatorStatus: IndicatorStatus;
}

export const MenuIcon = forwardRef<HTMLDivElement, MenuIconProps>(
  (props, ref) => {
    return (
      <StyledButton
        setRef={ref}
        onClick={props.onClick}
        kind="secondary"
        size="small"
        title="Open Connect My Computer"
      >
        <Laptop size="medium" />
        {props.indicatorStatus === 'error' ? (
          <StyledWarning />
        ) : (
          indicatorStatusToStyledStatus(props.indicatorStatus)
        )}
      </StyledButton>
    );
  }
);

const indicatorStatusToStyledStatus = (
  indicatorStatus: '' | 'processing' | 'success'
): JSX.Element => {
  return (
    <StyledStatus
      status={indicatorStatus}
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

        animation: blink 1.4s ease-in-out;
        animation-iteration-count: ${props => {
          const hasFinished =
            props.status === 'success' || props.status === 'error';
          return hasFinished ? '0' : 'infinite';
        }};
        visibility: ${props => (props.status === '' ? 'hidden' : 'visible')};
        background: ${props => getIndicatorColor(props.status, props.theme)};
      `}
    />
  );
};

function getIndicatorColor(
  status: 'processing' | 'success',
  theme: any
): string {
  switch (status) {
    case 'processing':
    case 'success':
      return theme.colors.success;
  }
}

const StyledButton = styled(Button)`
  position: relative;
  background: ${props => props.theme.colors.spotBackground[0]};
  padding: 0;
  width: ${props => props.theme.space[5]}px;
  height: ${props => props.theme.space[5]}px;
`;

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

const StyledWarning = styled(Warning).attrs({
  size: 'small',
  color: 'error.main',
})`
  position: absolute;
  top: -6px;
  right: -6px;
  z-index: 1;

  > svg {
    width: 14px;
    height: 14px;
  }
`;
