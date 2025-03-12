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

import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';
import { Platform } from 'teleterm/mainProcess/types';
import { makeAcl, makeLoggedInUser } from 'teleterm/services/tshd/testHelpers';

import {
  ConnectMyComputerAccess,
  ConnectMyComputerAccessNoAccess,
  getConnectMyComputerAccess,
} from './access';

const testCases: {
  name: string;
  platform: Platform;
  canCreateToken: boolean;
  expect: ConnectMyComputerAccess['status'];
  expectReason?: ConnectMyComputerAccessNoAccess['reason'];
}[] = [
  {
    name: 'access is granted when OS is darwin and can create token',
    platform: 'darwin',
    canCreateToken: true,
    expect: 'ok',
  },
  {
    name: 'access is granted when OS is  linux and can create token',
    platform: 'linux',
    canCreateToken: true,
    expect: 'ok',
  },
  {
    name: 'access is not granted when OS is windows and can create token',
    platform: 'win32',
    canCreateToken: true,
    expect: 'no-access',
    expectReason: 'unsupported-platform',
  },
  {
    name: 'access is not granted when OS is darwin and cannot create token',
    platform: 'darwin',
    canCreateToken: false,
    expect: 'no-access',
    expectReason: 'insufficient-permissions',
  },
];

test.each(testCases)('$name', testCase => {
  const loggedInUser = makeLoggedInUser({
    acl: makeAcl({
      tokens: {
        create: testCase.canCreateToken,
        edit: false,
        list: false,
        use: false,
        read: false,
        delete: false,
      },
    }),
  });
  const runtimeSettings = makeRuntimeSettings({ platform: testCase.platform });

  const access = getConnectMyComputerAccess(loggedInUser, runtimeSettings);
  expect(access.status).toEqual(testCase.expect);
  if (testCase.expectReason) {
    // eslint-disable-next-line jest/no-conditional-expect
    expect((access as ConnectMyComputerAccessNoAccess).reason).toEqual(
      testCase.expectReason
    );
  }
});
