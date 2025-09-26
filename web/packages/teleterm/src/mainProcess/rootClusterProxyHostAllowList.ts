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

import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import { proxyHostToBrowserProxyHost } from 'teleterm/services/tshd/cluster';
import { Logger } from 'teleterm/types';

/**
 * Produces a list of proxy and SSO hosts of root clusters. This enables us to
 * open links to Web UIs of clusters from within Connect.
 *
 * The port part of a proxy host is dropped if the port is 443. See `proxyHostToBrowserProxyHost` for
 * more details.
 */
export function makeRootClusterProxyHostAllowList(
  clusters: Cluster[],
  logger: Logger
): Set<string> {
  return new Set(
    clusters
      .flatMap(c => {
        if (!c.proxyHost) {
          return;
        }
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
