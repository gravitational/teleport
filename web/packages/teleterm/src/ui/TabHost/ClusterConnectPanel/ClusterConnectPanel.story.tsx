/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
  makeLoggedInUser,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import { ClusterConnectPanel } from './ClusterConnectPanel';

export default {
  title: 'Teleterm/ClusterConnectPanel',
};

const profileStatusError =
  'No YubiKey device connected with serial number 14358031. Connect the device and try again.';
const clusterOrange = makeRootCluster({
  name: 'orange',
  loggedInUser: makeLoggedInUser({
    name: 'bob',
    roles: ['access', 'editor'],
    sshLogins: ['root'],
  }),
  uri: '/clusters/orange',
});
const clusterViolet = makeRootCluster({
  name: 'violet',
  loggedInUser: makeLoggedInUser({ name: 'sammy' }),
  uri: '/clusters/violet',
});

export const Empty = () => {
  return (
    <MockAppContextProvider>
      <ClusterConnectPanel />
    </MockAppContextProvider>
  );
};

export const WithClusters = () => {
  const ctx = new MockAppContext();
  ctx.addRootCluster(clusterOrange);
  ctx.addRootCluster(clusterViolet);

  return (
    <MockAppContextProvider appContext={ctx}>
      <ClusterConnectPanel />;
    </MockAppContextProvider>
  );
};

export const WithErrors = () => {
  const ctx = new MockAppContext();
  ctx.addRootCluster(
    makeRootCluster({
      ...clusterOrange,
      profileStatusError,
    })
  );
  ctx.addRootCluster(clusterViolet);
  return (
    <MockAppContextProvider appContext={ctx}>
      <ClusterConnectPanel />;
    </MockAppContextProvider>
  );
};
