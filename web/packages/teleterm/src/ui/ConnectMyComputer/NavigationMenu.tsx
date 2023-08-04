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

import React, { useRef, useState } from 'react';
import { Menu, MenuItem } from 'design';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ClusterUri } from 'teleterm/ui/uri';
import { useWorkspaceContext } from 'teleterm/ui/Documents';

import { NavigationMenuIcon } from './NavigationMenuIcon';
import { canUseConnectMyComputer } from './permissions';
import { useConnectMyComputerContext } from './connectMyComputerContext';

interface NavigationMenuProps {
  clusterUri: ClusterUri;
}

export function NavigationMenu(props: NavigationMenuProps) {
  const iconRef = useRef();
  const [isMenuOpened, setIsMenuOpened] = useState(false);
  const appCtx = useAppContext();
  const { documentsService, rootClusterUri } = useWorkspaceContext();
  const { isSetupDoneAttempt, state } = useConnectMyComputerContext();
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

  return (
    <>
      <NavigationMenuIcon
        agentState={state}
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
      >
        {isSetupDoneAttempt.status === 'success' && (
          <>
            {!isSetupDoneAttempt.data ? (
              <MenuItem onClick={openSetupDocument}>Connect computer</MenuItem>
            ) : (
              <MenuItem onClick={openStatusDocument}>Manage agent</MenuItem>
            )}
          </>
        )}
      </Menu>
    </>
  );
}
