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

import { Attempt } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ClusterUri } from 'teleterm/ui/uri';
import { useWorkspaceContext } from 'teleterm/ui/Documents';

import { canUseConnectMyComputer } from './permissions';
import {
  AgentState,
  useConnectMyComputerContext,
} from './connectMyComputerContext';

interface NavigationMenuProps {
  clusterUri: ClusterUri;
}

export function NavigationMenu(props: NavigationMenuProps) {
  const iconRef = useRef();
  const [isMenuOpened, setIsMenuOpened] = useState(false);
  const appCtx = useAppContext();
  const { documentsService, rootClusterUri } = useWorkspaceContext();
  const { isAgentConfiguredAttempt, state } = useConnectMyComputerContext();
  // DocumentCluster renders this component only if the cluster exists.
  const cluster = appCtx.clustersService.findCluster(props.clusterUri);

  // Don't show the navigation icon for leaf clusters.
  if (cluster.leaf) {
    return null;
  }

  const rootCluster = cluster;

  function toggleMenu() {
    setIsMenuOpened(wasOpened => !wasOpened);
  }

  function openSetupDocument(): void {
    documentsService.openConnectMyComputerSetupDocument({
      rootClusterUri,
    });
    setIsMenuOpened(false);
  }

  function openStatusDocument(): void {
    documentsService.openConnectMyComputerStatusDocument({
      rootClusterUri,
    });
    setIsMenuOpened(false);
  }

  if (
    !canUseConnectMyComputer(
      rootCluster,
      appCtx.configService,
      appCtx.mainProcessClient.getRuntimeSettings()
    )
  ) {
    return null;
  }

  const setupMenuItem = (
    <MenuItem onClick={openSetupDocument}>Connect computer</MenuItem>
  );
  const statusMenuItem = (
    <MenuItem onClick={openStatusDocument}>
      {isInErrorState(state, isAgentConfiguredAttempt) && (
        <Warning size="small" color="error.main" mr={1} />
      )}
      Manage agent
    </MenuItem>
  );

  return (
    <>
      <MenuIcon
        agentState={state}
        isSetupDoneAttempt={isAgentConfiguredAttempt}
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

interface MenuIconProps {
  onClick(): void;
  agentState: AgentState;
  isSetupDoneAttempt: Attempt<boolean>;
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
        {getStateIndicator(props.agentState, props.isSetupDoneAttempt)}
      </StyledButton>
    );
  }
);

function getStateIndicator(
  agentState: AgentState,
  isSetupDoneAttempt: Attempt<boolean>
): JSX.Element {
  if (isInErrorState(agentState, isSetupDoneAttempt)) {
    return <StyledStatus bg="error.main" />;
  }
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
  }
}

function isInErrorState(
  agentState: AgentState,
  isSetupDoneAttempt: Attempt<boolean>
): boolean {
  if (isSetupDoneAttempt.status === 'error') {
    return true;
  }
  switch (agentState.status) {
    case 'error': {
      return true;
    }
    case 'exited': {
      if (!agentState.exitedSuccessfully) {
        return true;
      }
    }
  }
  return false;
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
