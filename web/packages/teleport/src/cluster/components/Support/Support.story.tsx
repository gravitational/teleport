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

import React from 'react';
import Support from './Support';
import TeleportContext from 'teleport/teleportContext';
import TeleportContextProvider from 'teleport/teleportContextProvider';
import cfg from 'teleport/config';

export default {
  title: 'Teleport/Support',
};

export const SupportOSS = () => {
  cfg.isEnterprise = false;
  const ctx = new TeleportContext();
  ctx.storeUser.state = state;

  return (
    <TeleportContextProvider value={ctx}>
      <Support />
    </TeleportContextProvider>
  );
};

export const SupportEnterprise = () => {
  cfg.isEnterprise = true;
  const ctx = new TeleportContext();
  ctx.storeUser.state = state;

  return (
    <TeleportContextProvider value={ctx}>
      <Support />
    </TeleportContextProvider>
  );
};

const cluster = {
  clusterId: 'test cluster name',
  lastConnected: null,
  connectedText: null,
  status: null,
  url: 'test/url',
  nodeCount: 50,
  publicURL: 'test/public/url',
  authVersion: '5.0.0',
  proxyVersion: '6.0.0',
};

const state = {
  authType: null,
  acl: null,
  username: null,
  cluster,
};
