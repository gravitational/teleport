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

import React from 'react';

import { Box, ButtonBorder, Card, Text } from 'design';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { getUserWithClusterName } from 'teleterm/ui/utils';
import { RootClusterUri } from 'teleterm/ui/uri';

export function RecentClusters() {
  const ctx = useAppContext();

  ctx.clustersService.useState();

  const rootClusters = ctx.clustersService
    .getClusters()
    .filter(c => !c.leaf)
    .map(cluster => ({
      userWithClusterName: getUserWithClusterName({
        userName: cluster.loggedInUser?.name,
        clusterName: cluster.name,
      }),
      uri: cluster.uri,
    }));

  function connect(clusterUri: RootClusterUri): void {
    ctx.workspacesService.setActiveWorkspace(clusterUri);
  }

  if (!rootClusters.length) {
    return null;
  }

  return (
    <Card p={3} maxWidth="480px" m="auto">
      <Text bold fontSize={3} mb={1}>
        Recent clusters
      </Text>
      <Box as="ul" p={0} m={0} maxHeight="110px" overflow="auto">
        {rootClusters.map((cluster, index) => (
          <Box
            as="li"
            mb={index !== rootClusters.length ? 1 : 0}
            key={cluster.uri}
            css={`
              display: flex;
              justify-content: space-between;
            `}
          >
            <Text
              color="text.main"
              mr={2}
              title={cluster.userWithClusterName}
              css={`
                white-space: nowrap;
              `}
            >
              {cluster.userWithClusterName}
            </Text>
            <ButtonBorder
              size="small"
              onClick={() => connect(cluster.uri)}
              title={`Connect to ${cluster.userWithClusterName}`}
            >
              Connect
            </ButtonBorder>
          </Box>
        ))}
      </Box>
    </Card>
  );
}
