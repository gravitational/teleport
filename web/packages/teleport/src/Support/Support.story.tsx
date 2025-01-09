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

import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';
import { ContentMinWidth } from 'teleport/Main/Main';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { Props, Support } from './Support';

export default {
  title: 'Teleport/Support',
};

const Provider = ({ children }) => (
  <ContextProvider ctx={ctx}>
    <ContentMinWidth>
      <MemoryRouter>{children}</MemoryRouter>
    </ContentMinWidth>
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
    <Support {...props} isCloud={true} />
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
  licenseExpiryDateText: '2027-05-09 06:52:58',
};
