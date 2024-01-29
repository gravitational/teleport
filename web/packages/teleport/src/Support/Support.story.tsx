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
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';

import { createTeleportContext } from 'teleport/mocks/contexts';

import { Props, Support } from './Support';

export default {
  title: 'Teleport/Support',
};

const Provider = ({ children }) => (
  <ContextProvider ctx={ctx}>
    <MemoryRouter>{children}</MemoryRouter>
  </ContextProvider>
);

export const SupportOSS = () => (
  <Provider>
    <Support {...props} />
  </Provider>
);

export const SupportOSSWithCTA = () => (
  <Provider>
    <Support {...props} showPremiumSupportCTA={true} />
  </Provider>
);

export const SupportCloud = () => (
  <Provider>
    <Support {...props} isCloud={true} />;
  </Provider>
);

export const SupportEnterprise = () => (
  <Provider>
    <Support {...props} isEnterprise={true} />
  </Provider>
);

export const SupportEnterpriseWithCTA = () => (
  <Provider>
    <Support {...props} isEnterprise={true} showPremiumSupportCTA={true} />
  </Provider>
);

export const SupportWithTunnelAddress = () => (
  <Provider>
    <Support {...props} tunnelPublicAddress="localhost:11005"></Support>
  </Provider>
);

const ctx = createTeleportContext();

const props: Props = {
  clusterId: 'test',
  authVersion: '13.4.0-dev',
  publicURL: 'localhost:3080',
  isEnterprise: false,
  isCloud: false,
  tunnelPublicAddress: null,
  showPremiumSupportCTA: false,
};
