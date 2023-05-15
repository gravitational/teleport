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

import { ContextProvider } from 'teleport';

import { createTeleportContext } from 'teleport/mocks/contexts';

import { Props, Support } from './Support';

export default {
  title: 'Teleport/Support',
};

export const SupportOSS = () => (
  <ContextProvider ctx={ctx}>
    <Support {...props} />
  </ContextProvider>
);

export const SupportCloud = () => (
  <ContextProvider ctx={ctx}>
    <Support {...props} isCloud={true} />;
  </ContextProvider>
);

export const SupportEnterprise = () => (
  <ContextProvider ctx={ctx}>
    <Support {...props} isEnterprise={true} />
  </ContextProvider>
);

export const SupportWithCTA = () => (
  <ContextProvider ctx={ctx}>
    <Support {...props} isEnterprise={true} showPremiumSupportCTA={true} />
  </ContextProvider>
);

export const SupportWithTunnelAddress = () => (
  <ContextProvider ctx={ctx}>
    <Support {...props} tunnelPublicAddress="localhost:11005"></Support>
  </ContextProvider>
);

const ctx = createTeleportContext();

const props: Props = {
  clusterId: 'test',
  authVersion: '4.4.0-dev',
  publicURL: 'localhost:3080',
  isEnterprise: false,
  isCloud: false,
  tunnelPublicAddress: null,
  showPremiumSupportCTA: false,
};
