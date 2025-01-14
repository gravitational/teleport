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

import styled from 'styled-components';

import { Box, ButtonPrimary, Flex, H1, ResourceIcon, Text } from 'design';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import { RecentClusters } from './RecentClusters';

export function ClusterConnectPanel() {
  const ctx = useAppContext();

  function handleConnect() {
    ctx.commandLauncher.executeCommand('cluster-connect', {});
  }

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
          <H1 mb={2}>Connect a Cluster</H1>
          <Text color="text.slightlyMuted" mb={3} textAlign="center">
            Connect an existing Teleport cluster <br /> to start using Teleport
            Connect.
          </Text>
          <ButtonPrimary size="large" onClick={handleConnect}>
            Connect
          </ButtonPrimary>
        </Flex>
        <RecentClusters />
      </Box>
    </ScrollingContainer>
  );
}

const ScrollingContainer = styled(Flex)`
  background: ${props => props.theme.colors.levels.sunken};
  width: 100%;
  overflow: auto;
`;
