/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { ipcMain } from 'electron';

import { isAbortError } from 'shared/utils/abortError';

import { MainProcessIpc } from 'teleterm/mainProcess/types';
import { TshdClient } from 'teleterm/services/tshd';
import { cloneAbortSignal } from 'teleterm/services/tshd/cloneableClient';
import { proxyHostToBrowserProxyHost } from 'teleterm/services/tshd/cluster';
import * as tshd from 'teleterm/services/tshd/types';
import { Logger } from 'teleterm/types';

export type RootClusterProxyHostAllowList = Set<string>;

/**
 * Refreshes the allow list whenever the renderer process notifies the main process that the list of
 * clusters in ClustersService got updated.
 *
 * The allow list includes proxy hosts of root clusters. This enables us to open links to Web UIs of
 * clusters from within Connect.
 *
 * The port part of a proxy host is dropped if the port is 443. See proxyHostToBrowserProxyHost for
 * more details.
 */
export function manageRootClusterProxyHostAllowList({
  tshdClient,
  logger,
  allowList,
}: {
  tshdClient: TshdClient;
  logger: Logger;
  allowList: RootClusterProxyHostAllowList;
}) {
  let abortController: AbortController;

  const refreshAllowList = async () => {
    // Allow only one call to be in progress. This ensures that on subsequent calls to
    // refreshAllowList, we always store only the most recent version of the list.
    abortController?.abort();
    abortController = new AbortController();

    let rootClusters: tshd.Cluster[];
    try {
      const { response } = await tshdClient.listRootClusters(
        {},
        { abort: cloneAbortSignal(abortController.signal) }
      );
      rootClusters = response.clusters;
    } catch (error) {
      if (isAbortError(error)) {
        // Ignore abort errors. They will be logged by the gRPC client middleware.
        return;
      }

      logger.error('Could not fetch root clusters', error);
      // Return instead of throwing as there's nothing else we can do with the error at this place
      // in the program.
      return;
    }

    allowList.clear();
    for (const rootCluster of rootClusters) {
      if (rootCluster.proxyHost) {
        let browserProxyHost: string;
        try {
          browserProxyHost = proxyHostToBrowserProxyHost(rootCluster.proxyHost);
          allowList.add(browserProxyHost);
        } catch (error) {
          logger.error(
            'Ran into an error when converting proxy host to browser proxy host',
            error
          );
        }
      }

      // Allow the SSO host for SSO login/mfa redirects.
      if (rootCluster.ssoHost) {
        let browserSsoHost: string;
        try {
          browserSsoHost = proxyHostToBrowserProxyHost(rootCluster.ssoHost);
          allowList.add(browserSsoHost);
        } catch (error) {
          logger.error(
            'Ran into an error when converting sso host to browser sso host',
            error
          );
        }
      }
    }
  };

  refreshAllowList();

  ipcMain.on(MainProcessIpc.RefreshClusterList, () => {
    refreshAllowList();
  });
}
