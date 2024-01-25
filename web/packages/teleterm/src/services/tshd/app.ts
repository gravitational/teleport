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

import { App, Cluster } from 'teleterm/services/tshd/types';

/** Returns a URL that can be used to open the app in a browser. */
export function getWebAppLaunchUrl({
  app,
  cluster,
  rootCluster,
}: {
  app: App;
  rootCluster: Cluster;
  cluster: Cluster;
}): string {
  const { fqdn, publicAddr } = app;

  const canCreateUrl =
    rootCluster.proxyHost && fqdn && cluster?.name && publicAddr;

  if (!canCreateUrl) {
    return '';
  }
  return `https://${rootCluster.proxyHost}/web/launch/${fqdn}/${cluster.name}/${publicAddr}`;
}

export function isWebApp(app: App): boolean {
  if (app.samlApp || app.awsConsole) {
    return false;
  }
  return (
    app.endpointUri.startsWith('http://') ||
    app.endpointUri.startsWith('https://')
  );
}
