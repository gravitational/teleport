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

import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { render } from 'design/utils/testing';
import { TrustedDeviceRequirement } from 'gen-proto-ts/teleport/legacy/types/trusted_device_requirement_pb';

import {
  makeLoggedInUser,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import { IdentityContainer } from './Identity';

/* oxlint-disable jest/no-standalone-expect */

test.each([
  {
    name: 'device enrollment confirmation is visible if device is trusted',
    user: makeLoggedInUser({
      isDeviceTrusted: true,
    }),
    expect: async () => {
      expect(
        await screen.findByText(/access secured with device trust/i)
      ).toBeVisible();
    },
  },
  {
    name: 'warning about required enrollment is visible when device trust is required but device is not enrolled',
    user: makeLoggedInUser({
      isDeviceTrusted: false,
      trustedDeviceRequirement: TrustedDeviceRequirement.REQUIRED,
    }),
    expect: async () => {
      expect(
        await screen.findByText(/Full access requires a trusted device/)
      ).toBeVisible();
    },
  },
  {
    name: 'no message is visible when device trust is not required and device is not enrolled',
    user: makeLoggedInUser({
      isDeviceTrusted: false,
      trustedDeviceRequirement: TrustedDeviceRequirement.NOT_REQUIRED,
    }),
    expect: async () => {
      expect(
        screen.queryByText(/access secured with device trust/i)
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText(/Full access requires a trusted device/)
      ).not.toBeInTheDocument();
    },
  },
])('$name', async testCase => {
  const appContext = new MockAppContext();
  appContext.addRootCluster(
    makeRootCluster({
      loggedInUser: testCase.user,
    })
  );

  render(
    <MockAppContextProvider appContext={appContext}>
      <IdentityContainer />
    </MockAppContextProvider>
  );

  await userEvent.click(await screen.findByTitle(/Open Profiles/));

  await testCase.expect();
});

test('roles list remembers it was expanded', async () => {
  const appContext = new MockAppContext();
  appContext.addRootCluster(
    makeRootCluster({
      loggedInUser: makeLoggedInUser({ roles: ['requests-reviewer'] }),
    })
  );

  render(
    <MockAppContextProvider appContext={appContext}>
      <IdentityContainer />
    </MockAppContextProvider>
  );

  await toggleProfiles();

  const roleItemBeforeExpand = await screen.findByText('requests-reviewer');
  expect(roleItemBeforeExpand).not.toBeVisible();

  await userEvent.click(await screen.findByText(/Roles/));
  expect(await screen.findByText('requests-reviewer')).toBeVisible();

  // Close and open again.
  await toggleProfiles();
  expect(screen.queryByText('requests-reviewer')).not.toBeInTheDocument();

  await toggleProfiles();
  expect(await screen.findByText('requests-reviewer')).toBeVisible();
});

test('shows each identity row with the correct profile status and action', async () => {
  const appContext = new MockAppContext();
  const activeCluster = makeRootCluster({
    uri: '/clusters/active',
    loggedInUser: makeLoggedInUser({ name: 'active-user' }),
  });
  const connectedCluster = makeRootCluster({
    uri: '/clusters/connected',
    loggedInUser: makeLoggedInUser({
      name: 'alice',
    }),
  });
  const expiredCluster = makeRootCluster({
    uri: '/clusters/expired',
    connected: false,
    loggedInUser: makeLoggedInUser({
      name: 'expired-user',
    }),
  });
  const disconnectedCluster = makeRootCluster({
    uri: '/clusters/disconnected',
    connected: false,
    loggedInUser: undefined,
  });
  const executeCommandSpy = jest.spyOn(
    appContext.commandLauncher,
    'executeCommand'
  );
  const forgetClusterSpy = jest.spyOn(
    appContext.mockMainProcessClient,
    'forgetCluster'
  );

  appContext.addRootCluster(activeCluster);
  appContext.addRootCluster(connectedCluster, { noActivate: true });
  appContext.addRootCluster(expiredCluster, { noActivate: true });
  appContext.addRootCluster(disconnectedCluster, { noActivate: true });
  appContext.workspacesService.addWorkspace({
    uri: '/clusters/orphan',
    proxyHost: 'this-is-orphaned-cluster.com',
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <IdentityContainer />
    </MockAppContextProvider>
  );

  await toggleProfiles();

  expect(
    within(await screen.findByTitle('Switch to connected')).getByText('alice')
  ).toBeVisible();
  expect(
    within(await screen.findByTitle('Switch to expired')).getByText(
      'expired-user · Expired session'
    )
  ).toBeVisible();
  expect(
    within(await screen.findByTitle('Switch to disconnected')).getByText(
      'Not logged in'
    )
  ).toBeVisible();
  expect(
    within(await screen.findByTitle('Switch to orphan')).getByText(
      'Previously used'
    )
  ).toBeVisible();

  await userEvent.click(screen.getByTitle('Log out from connected'));
  expect(executeCommandSpy).toHaveBeenCalledWith('cluster-logout', {
    clusterUri: '/clusters/connected',
  });

  await toggleProfiles();
  await userEvent.click(screen.getByTitle('Forget disconnected'));
  expect(forgetClusterSpy).toHaveBeenCalledWith('/clusters/disconnected');

  await toggleProfiles();
  await userEvent.click(screen.getByTitle('Forget expired'));
  expect(forgetClusterSpy).toHaveBeenCalledWith('/clusters/expired');

  await toggleProfiles();
  await userEvent.click(screen.getByTitle('Forget orphan'));
  expect(forgetClusterSpy).toHaveBeenCalledWith('/clusters/orphan');
});

async function toggleProfiles() {
  await userEvent.click(await screen.findByTitle(/Open Profiles/));
}
