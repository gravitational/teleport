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

import { dialog, shell, WebContents } from 'electron';

import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import { proxyHostToBrowserProxyHost } from 'teleterm/services/tshd/cluster';
import { Logger, RuntimeSettings } from 'teleterm/types';

import { ClusterStore } from './clusterStore';

/**
 * Limits navigation capabilities to reduce the attack surface.
 * See TEL-Q122-19 from "Teleport Core Testing Q1 2022" security audit.
 *
 * See also points 12, 13 and 14 from the Electron's security tutorial.
 * https://github.com/electron/electron/blob/v17.2.0/docs/tutorial/security.md#12-verify-webview-options-before-creation
 */
export function registerNavigationHandlers(
  webContents: Pick<WebContents, 'setWindowOpenHandler' | 'on'>,
  settings: Pick<RuntimeSettings, 'dev'>,
  clusterStore: Pick<ClusterStore, 'getRootClusters'>,
  logger: Logger
): void {
  webContents.on('will-navigate', (event, navigationUrl) => {
    // Allow reloading the renderer app in dev mode.
    if (settings.dev && new URL(navigationUrl).host === 'localhost:8080') {
      return;
    }
    logger.warn(`Navigation to ${navigationUrl} blocked by 'will-navigate'`);
    event.preventDefault();
  });

  // The usage of webview is blocked by default, but let's include the handler just in case.
  // https://github.com/electron/electron/blob/v17.2.0/docs/api/webview-tag.md#enabling
  webContents.on('will-attach-webview', (event, _, params) => {
    logger.warn(
      `Opening a webview to ${params.src} blocked by 'will-attach-webview'`
    );
    event.preventDefault();
  });

  // Open links in the browser.
  webContents.setWindowOpenHandler(details => {
    const url = new URL(details.url);

    if (isUrlSafe(url, clusterStore, logger)) {
      shell.openExternal(url.toString());
    } else {
      logger.warn(
        `Opening a new window to ${url} blocked by 'setWindowOpenHandler'`
      );
      dialog.showErrorBox(
        'Cannot open this link',
        'The domain does not match any of the allowed domains. Check main.log for more details.'
      );
    }

    return { action: 'deny' };
  });
}

function isUrlSafe(
  url: URL,
  clusterStore: Pick<ClusterStore, 'getRootClusters'>,
  logger: Logger
): boolean {
  if (url.protocol !== 'https:') {
    return false;
  }
  if (url.host === 'goteleport.com') {
    return true;
  }
  if (url.host === 'github.com' && url.pathname.startsWith('/gravitational/')) {
    return true;
  }

  const rootClusterProxyHostAllowList = makeRootClusterProxyHostAllowList(
    clusterStore.getRootClusters(),
    logger
  );

  // Allow opening links to the Web UIs of root clusters currently added in the app.
  if (rootClusterProxyHostAllowList.has(url.host)) {
    return true;
  }
}

/**
 * Produces a list of proxy and SSO hosts of root clusters. This enables us to
 * open links to Web UIs of clusters from within Connect.
 *
 * The port part of a proxy host is dropped if the port is 443. See `proxyHostToBrowserProxyHost` for
 * more details.
 */
function makeRootClusterProxyHostAllowList(
  clusters: Cluster[],
  logger: Logger
): Set<string> {
  return new Set(
    clusters
      .flatMap(c => {
        let browserProxyHost: string;
        if (c.proxyHost) {
          try {
            browserProxyHost = proxyHostToBrowserProxyHost(c.proxyHost);
          } catch (error) {
            logger.error(
              'Ran into an error when converting proxy host to browser proxy host',
              error
            );
          }
        }

        // Allow the SSO host for SSO login/mfa redirects.
        let browserSsoHost: string;
        if (c.ssoHost) {
          try {
            browserSsoHost = proxyHostToBrowserProxyHost(c.ssoHost);
          } catch (error) {
            logger.error(
              'Ran into an error when converting sso host to browser sso host',
              error
            );
          }
        }
        return [browserProxyHost, browserSsoHost];
      })
      .filter(Boolean)
  );
}
