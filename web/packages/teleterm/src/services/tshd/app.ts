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

import {
  App,
  PortRange,
  RouteToApp,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/app_pb';
import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

/** Returns a URL that opens the web app in the browser. */
export function getWebAppLaunchUrl({
  app,
  cluster,
  rootCluster,
}: {
  app: App;
  rootCluster: Cluster;
  cluster: Cluster;
}): string {
  if (!isWebApp(app)) {
    return '';
  }

  const { fqdn, publicAddr } = app;
  return `https://${rootCluster.proxyHost}/web/launch/${fqdn}/${cluster.name}/${publicAddr}`;
}

/** Returns a URL that opens the AWS app in the browser. */
export function getAwsAppLaunchUrl({
  app,
  cluster,
  rootCluster,
  arn,
}: {
  app: App;
  rootCluster: Cluster;
  cluster: Cluster;
  arn: string;
}): string {
  if (!app.awsConsole) {
    return '';
  }

  const { fqdn, publicAddr } = app;
  return `https://${rootCluster.proxyHost}/web/launch/${fqdn}/${
    cluster.name
  }/${publicAddr}/${encodeURIComponent(arn)}`;
}

/** Returns a URL that triggers IdP-initiated SSO for SAML Application. */
export function getSamlAppSsoUrl({
  app,
  rootCluster,
}: {
  app: App;
  rootCluster: Cluster;
}): string {
  if (!app.samlApp) {
    return '';
  }
  return `https://${
    rootCluster.proxyHost
  }/enterprise/saml-idp/login/${encodeURIComponent(app.name)}`;
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

/**
 * Returns address with protocol which is an app protocol + a public address.
 * If the public address is empty, it falls back to the endpoint URI.
 *
 * Always empty for SAML applications.
 */
export function getAppAddrWithProtocol(source: App): string {
  const { publicAddr, endpointUri } = source;

  const isTcp = endpointUri && endpointUri.startsWith('tcp://');
  const isCloud = endpointUri && endpointUri.startsWith('cloud://');
  let addrWithProtocol = endpointUri;
  if (publicAddr) {
    if (isCloud) {
      addrWithProtocol = `cloud://${publicAddr}`;
    } else if (isTcp) {
      addrWithProtocol = `tcp://${publicAddr}`;
    } else {
      addrWithProtocol = `https://${publicAddr}`;
    }
  }

  return addrWithProtocol;
}

export const portRangeSeparator = '-';

export const formatPortRange = (portRange: PortRange): string =>
  portRange.endPort === 0
    ? portRange.port.toString()
    : `${portRange.port}${portRangeSeparator}${portRange.endPort}`;

export const publicAddrWithTargetPort = (routeToApp: RouteToApp): string =>
  routeToApp.targetPort
    ? `${routeToApp.publicAddr}:${routeToApp.targetPort}`
    : routeToApp.publicAddr;
