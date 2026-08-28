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

import { useEffect, useRef } from 'react';
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

import { NullKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation/KeyboardArrowsNavigation';
import {
  TshHomeMigrationBanner,
  useIdentity,
  IdentityList,
} from 'teleterm/ui/TopBar/Identity';

export function ClusterConnectPanel() {
  const {
    // ClusterConnectPanel is rendered only when there is no active workspace, so
    // the hook's "other workspaces" are all available workspaces.
    otherWorkspaces: availableWorkspaces,
    logout,
    forget,
    addCluster,
    changeWorkspace,
  } = useIdentity();

  const containerRef = useRef<HTMLDivElement>(null);

  // Focus the first item.
  const hasAnyWorkspaces = !!availableWorkspaces.length;
  useEffect(() => {
    if (hasAnyWorkspaces) {
      containerRef.current.querySelector('li').focus();
    }
  }, [hasAnyWorkspaces]);

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
          {hasAnyWorkspaces ? (
            <Flex flexDirection="column">
              <H2>Clusters</H2>
              <P2 color="text.slightlyMuted" mb={2}>
                Log in to a cluster to use Teleport Connect.
              </P2>
              {/* Apply the same styling as used for the cluster items below. */}
              <TshHomeMigrationBanner
                css={`
                  margin-bottom: ${p => p.theme.space[1]}px;
                  border-radius: ${p => p.theme.radii[2]}px;
                  padding: ${p => p.theme.space[2]}px;
                `}
              />
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
                  <IdentityList
                    items={availableWorkspaces}
                    onAdd={addCluster}
                    onSelect={changeWorkspace}
                    onLogout={logout}
                    onForget={forget}
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
              <ButtonPrimary size="large" onClick={addCluster}>
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
