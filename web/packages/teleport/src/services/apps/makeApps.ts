/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import cfg from 'teleport/config';

import { App } from './types';

export default function makeApp(json: any): App {
  json = json || {};
  const {
    name = '',
    description = '',
    uri = '',
    publicAddr = '',
    clusterId = '',
    fqdn = '',
    awsConsole = false,
    friendlyName = '',
  } = json;

  const canCreateUrl = fqdn && clusterId && publicAddr;
  const launchUrl = canCreateUrl
    ? cfg.getAppLauncherRoute({ fqdn, clusterId, publicAddr })
    : '';
  const id = `${clusterId}-${name}-${publicAddr || uri}`;
  const labels = json.labels || [];
  const awsRoles = json.awsRoles || [];
  const userGroups = json.userGroups || [];

  const isTcp = uri && uri.startsWith('tcp://');
  const isCloud = uri && uri.startsWith('cloud://');

  let addrWithProtocol = uri;
  if (publicAddr) {
    if (isCloud) {
      addrWithProtocol = `cloud://${publicAddr}`;
    } else if (isTcp) {
      addrWithProtocol = `tcp://${publicAddr}`;
    } else {
      addrWithProtocol = `https://${publicAddr}`;
    }
  }

  return {
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
    isCloudOrTcpEndpoint: isTcp || isCloud,
    addrWithProtocol,
    friendlyName,
    userGroups
  };
}
