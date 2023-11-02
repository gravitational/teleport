/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';
import { Platform } from 'teleterm/mainProcess/types';
import { makeLoggedInUser } from 'teleterm/services/tshd/testHelpers';

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
    acl: {
      tokens: {
        create: testCase.canCreateToken,
        edit: false,
        list: false,
        use: false,
        read: false,
        pb_delete: false,
      },
    },
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
