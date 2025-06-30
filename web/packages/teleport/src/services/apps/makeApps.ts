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

import { AppSubKind } from 'shared/services';
import { AwsRole } from 'shared/services/apps';

import cfg from 'teleport/config';

import { App, PermissionSet } from './types';

function getLaunchUrl({
  fqdn,
  clusterId,
  publicAddr,
  useAnyProxyPublicAddr,
}: {
  fqdn: string;
  clusterId: string;
  useAnyProxyPublicAddr: boolean;
  publicAddr: string;
}) {
  if (useAnyProxyPublicAddr) {
    return cfg.getAppLauncherRoute({
      fqdn,
    });
  }

  if (publicAddr && clusterId && fqdn) {
    return cfg.getAppLauncherRoute({ fqdn, publicAddr, clusterId });
  }

  return '';
}

export default function makeApp(json: any): App {
  json = json || {};
  const {
    name = '',
    description = '',
    uri = '',
    publicAddr = '',
    clusterId = '',
    fqdn = '',
    useAnyProxyPublicAddr = false,
    awsConsole = false,
    samlApp = false,
    friendlyName = '',
    requiresRequest,
    integration = '',
    samlAppPreset,
    subKind,
    samlAppLaunchUrls,
    mcp,
  } = json;

  const launchUrl = getLaunchUrl({
    fqdn,
    clusterId,
    publicAddr,
    useAnyProxyPublicAddr,
  });
  const id = `${clusterId}-${name}-${publicAddr || uri}`;
  const labels = json.labels || [];
  const awsRoles: AwsRole[] = json.awsRoles || [];
  const userGroups = json.userGroups || [];
  const permissionSets: PermissionSet[] = json.permissionSets || [];

  const isTcp = !!uri && uri.startsWith('tcp://');
  const isCloud = !!uri && uri.startsWith('cloud://');
  const isMCPStdio = !!uri && uri.startsWith('mcp+stdio://');

  let addrWithProtocol = uri;
  if (publicAddr) {
    if (isCloud) {
      addrWithProtocol = `cloud://${publicAddr}`;
    } else if (isTcp) {
      addrWithProtocol = `tcp://${publicAddr}`;
    } else if (isMCPStdio) {
      addrWithProtocol = `mcp+stdio://${publicAddr}`;
    } else if (subKind === AppSubKind.AwsIcAccount) {
      /** publicAddr for Identity Center account app is a URL with scheme. */
      addrWithProtocol = publicAddr;
    } else {
      addrWithProtocol = `https://${publicAddr}`;
    }
  }
  if (useAnyProxyPublicAddr) {
    addrWithProtocol = `https://${fqdn}`;
  }
  let samlAppSsoUrl = '';
  if (samlApp) {
    samlAppSsoUrl = `${cfg.baseUrl}/enterprise/saml-idp/login/${name}`;
  }

  return {
    kind: 'app',
    subKind,
    id,
    name,
    description,
    uri,
    publicAddr,
    labels,
    clusterId,
    fqdn,
    launchUrl,
    awsRoles,
    awsConsole,
    isTcp,
    isCloud,
    addrWithProtocol,
    useAnyProxyPublicAddr,
    friendlyName,
    userGroups,
    samlApp,
    samlAppPreset,
    samlAppSsoUrl,
    requiresRequest,
    integration,
    permissionSets,
    samlAppLaunchUrls,
    mcp,
  };
}
