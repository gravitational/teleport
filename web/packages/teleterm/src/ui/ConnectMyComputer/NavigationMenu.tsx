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

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ClusterUri } from 'teleterm/ui/uri';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { assertUnreachable } from 'teleterm/ui/utils';

import {
  CurrentAction,
  useConnectMyComputerContext,
} from './connectMyComputerContext';

interface NavigationMenuProps {
  clusterUri: ClusterUri;
}

/**
 * IndicatorStatus combines a couple of different states into a single enum which dictates the
 * decorative look of NavigationMenu.
 */
type IndicatorStatus = AttemptStatus;

export function NavigationMenu(props: NavigationMenuProps) {
  const iconRef = useRef();
  const [isMenuOpened, setIsMenuOpened] = useState(false);
  const appCtx = useAppContext();
  const { documentsService, rootClusterUri } = useWorkspaceContext();
  const { isAgentConfiguredAttempt, currentAction, canUse } =
    useConnectMyComputerContext();
  // DocumentCluster renders this component only if the cluster exists.
  const cluster = appCtx.clustersService.findCluster(props.clusterUri);
  const indicatorStatus = getIndicatorStatus(
    currentAction,
    isAgentConfiguredAttempt
  );

  // Don't show the navigation icon for leaf clusters.
  if (cluster.leaf || !canUse) {
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
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        onClose={() => setIsMenuOpened(false)}
        menuListCss={() =>
          css`
            width: 150px;
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
  isAgentConfiguredAttempt: Attempt<boolean>
): IndicatorStatus {
  if (isAgentConfiguredAttempt.status === 'error') {
    return 'error';
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
  } else {
    return currentAction.attempt.status;
  }
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
        {indicatorStatusToStyledStatus(props.indicatorStatus)}
      </StyledButton>
    );
  }
);

const indicatorStatusToStyledStatus = (
  indicatorStatus: IndicatorStatus
): JSX.Element => {
  switch (indicatorStatus) {
    case '': {
      return null;
    }
    case 'processing': {
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
    case 'error': {
      return <StyledStatus bg="error.main" />;
    }
    case 'success': {
      return <StyledStatus bg="success" />;
    }
  }
};

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
