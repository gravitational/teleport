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
    <Card p={3} maxWidth="480px" bg="levels.surfaceSecondary" m="auto">
      <Text bold fontSize={3} mb={1} color="light">
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
              color="text.primary"
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
