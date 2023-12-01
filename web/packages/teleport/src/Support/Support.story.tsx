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

export const SupportOSSWithCTA = () => (
  <ContextProvider ctx={ctx}>
    <Support {...props} showPremiumSupportCTA={true} />
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

export const SupportEnterpriseWithCTA = () => (
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
  authVersion: '13.4.0-dev',
  publicURL: 'localhost:3080',
  isEnterprise: false,
  isCloud: false,
  tunnelPublicAddress: null,
  showPremiumSupportCTA: false,
};
