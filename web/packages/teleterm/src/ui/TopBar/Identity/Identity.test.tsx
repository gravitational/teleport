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

import { screen } from '@testing-library/react';
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

test.each([
  {
    name: 'device enrollment confirmation is visible if device is trusted',
    user: makeLoggedInUser({
      isDeviceTrusted: true,
    }),
    expect: async () => {
      expect(
        await screen.findByText(/Access secured with device trust/)
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
        screen.queryByText(/Access secured with device trust/)
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
