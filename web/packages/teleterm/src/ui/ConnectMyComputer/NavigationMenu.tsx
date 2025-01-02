/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { forwardRef, useRef, useState } from 'react';
import styled, { css } from 'styled-components';

import { blink, Box, Button, Indicator, Menu, MenuItem } from 'design';
import { Laptop, Warning } from 'design/Icon';
import { Attempt, AttemptStatus } from 'shared/hooks/useAsync';

import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { assertUnreachable } from 'teleterm/ui/utils';

import {
  CurrentAction,
  useConnectMyComputerContext,
} from './connectMyComputerContext';

/**
 * IndicatorStatus combines a couple of different states into a single enum which dictates the
 * decorative look of NavigationMenu.
 *
 * 'not-configured' means that the user did not interact with the feature and thus no indicator
 * should be shown.
 * '' means that the user has interacted with the feature, but the agent is not currently running in
 * which case we display an empty circle, like we do next to the Connections icon when there's no
 * active connections.
 */
type IndicatorStatus = AttemptStatus | 'not-configured';

export function NavigationMenu() {
  const iconRef = useRef();
  const [isMenuOpened, setIsMenuOpened] = useState(false);
  const { documentsService, rootClusterUri } = useWorkspaceContext();
  const { isAgentConfiguredAttempt, currentAction, canUse } =
    useConnectMyComputerContext();

  if (!canUse) {
    return null;
  }

  const indicatorStatus = getIndicatorStatus(
    currentAction,
    isAgentConfiguredAttempt
  );

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
        menuListCss={() => css`
          display: flex;
          flex-direction: column;
        `}
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
  isAgentConfiguredAttempt: Attempt<boolean>
): IndicatorStatus {
  if (isAgentConfiguredAttempt.status === 'error') {
    return 'error';
  }
  const isAgentConfigured =
    isAgentConfiguredAttempt.status === 'success' &&
    isAgentConfiguredAttempt.data;

  // Returning 'not-configured' early means that the indicator won't be shown until the user
  // completes the setup.
  //
  // This is fine, as the setup has multiple steps but not all come from the context (and thus are
  // not ever assigned to currentAction). This means that if the indicator was shown during the
  // setup, it would not work reliably, as it would not reflect the progress of certain steps.
  if (!isAgentConfigured) {
    return 'not-configured';
  }

  if (currentAction.kind === 'observe-process') {
    switch (currentAction.agentProcessState.status) {
      case 'not-started': {
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

export const MenuIcon = forwardRef<HTMLButtonElement, MenuIconProps>(
  (props, ref) => {
    return (
      <StyledButton
        setRef={ref}
        onClick={props.onClick}
        intent="neutral"
        fill="filled"
        size="small"
        title="Open Connect My Computer"
        data-testid="connect-my-computer-icon"
      >
        <Laptop size="medium" />
        {props.indicatorStatus === 'error' ? (
          <StyledWarning />
        ) : (
          <StyledStatus status={props.indicatorStatus} />
        )}
      </StyledButton>
    );
  }
);

const StyledButton = styled(Button)`
  position: relative;
  padding: 0;
  width: ${props => props.theme.space[5]}px;
  height: ${props => props.theme.space[5]}px;
`;

const StyledStatus = styled(Box)<{ status: IndicatorStatus }>`
  position: absolute;
  top: -4px;
  right: -4px;
  z-index: 1;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);

  animation: ${blink} 1.4s ease-in-out;
  animation-iteration-count: ${props =>
    props.status === 'processing' ? 'infinite' : '0'};

  ${props => {
    const { status, theme } = props;
    // 'not-configured' means that the user did not interact with the feature and thus no indicator
    // should be shown.
    if (status === 'not-configured') {
      return { visibility: 'hidden' };
    }

    // '' means that the user has interacted with the feature, but the agent is not currently
    // running in which case we display an empty circle, like we do next to the Connections icon
    // when there's no active connections.
    if (status === '') {
      return {
        border: `1px solid ${theme.colors.text.slightlyMuted}`,
      };
    }

    if (status === 'processing' || status === 'success') {
      return { backgroundColor: theme.colors.success.main };
    }

    // 'error' status can be ignored as it's handled outside of StyledStatus.
  }}
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
