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

import { useMemo } from 'react';

import * as connectMyComputer from 'shared/connectMyComputer';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';

export function useAgentProperties(): {
  systemUsername: string;
  hostname: string;
  roleName: string;
  clusterName: string;
} {
  const { rootClusterUri } = useWorkspaceContext();
  const { clustersService, mainProcessClient } = useAppContext();
  const cluster = clustersService.findCluster(rootClusterUri);
  const { username: systemUsername, hostname } = useMemo(
    () => mainProcessClient.getRuntimeSettings(),
    [mainProcessClient]
  );

  return {
    systemUsername,
    hostname,
    roleName: cluster.loggedInUser
      ? connectMyComputer.getRoleNameForUser(cluster.loggedInUser.name)
      : '',
    clusterName: cluster.name,
  };
}
