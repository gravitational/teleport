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

import { useCallback, useEffect, useRef } from 'react';
import styled from 'styled-components';

import {
  Box,
  ButtonPrimary,
  Flex,
  H1,
  H2,
  P2,
  ResourceIcon,
  Text,
} from 'design';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { NullKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation/KeyboardArrowsNavigation';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import { ClusterList } from 'teleterm/ui/TopBar/Identity/IdentityList/IdentityList';
import { RootClusterUri } from 'teleterm/ui/uri';

export function ClusterConnectPanel() {
  const ctx = useAppContext();
  const clusters = useStoreSelector(
    'clustersService',
    useCallback(state => state.clusters, [])
  );
  const rootClusters = [...clusters.values()].filter(c => !c.leaf);
  function add(): void {
    ctx.commandLauncher.executeCommand('cluster-connect', {});
  }

  function connect(clusterUri: RootClusterUri): void {
    ctx.workspacesService.setActiveWorkspace(clusterUri);
  }

  const containerRef = useRef<HTMLDivElement>();

  // Focus the first item.
  const hasCluster = !!rootClusters.length;
  useEffect(() => {
    if (hasCluster) {
      containerRef.current.querySelector('li').focus();
    }
  }, [hasCluster]);

  return (
    <ScrollingContainer>
      <Box width="100%" m="auto" pb={3} pt={1} px={3}>
        <Flex
          minWidth="370px"
          pb={5}
          flexDirection="column"
          alignItems="center"
        >
          <ResourceIcon width="120px" name="server" mb={3} />
          {hasCluster ? (
            <Flex flexDirection="column">
              <H2>Clusters</H2>
              <P2 color="text.slightlyMuted" mb={2}>
                Log in to a cluster to use Teleport Connect.
              </P2>
              {/*Disable arrows navigation, it doesn't work well here,*/}
              {/*since it requires the container to be focused.*/}
              {/*The user can navigate with Tab.*/}
              <NullKeyboardArrowsNavigation>
                <Flex
                  maxWidth="450px"
                  ref={containerRef}
                  flexDirection="column"
                  css={`
                    li {
                      border-radius: ${p => p.theme.radii[2]}px;
                      padding: ${p => p.theme.space[2]}px;
                    }
                  `}
                >
                  <ClusterList
                    clusters={rootClusters}
                    onAdd={add}
                    onSelect={connect}
                  />
                </Flex>
              </NullKeyboardArrowsNavigation>
            </Flex>
          ) : (
            <>
              <H1 mb={2}>Connect a Cluster</H1>
              <Text color="text.slightlyMuted" mb={3} textAlign="center">
                Connect an existing Teleport cluster <br /> to start using
                Teleport Connect.
              </Text>
              <ButtonPrimary size="large" onClick={add}>
                Connect
              </ButtonPrimary>
            </>
          )}
        </Flex>
      </Box>
    </ScrollingContainer>
  );
}

const ScrollingContainer = styled(Flex)`
  background: ${props => props.theme.colors.levels.sunken};
  width: 100%;
  overflow: auto;
`;
