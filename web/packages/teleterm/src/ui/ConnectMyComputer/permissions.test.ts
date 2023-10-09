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
import {
  makeRootCluster,
  makeLoggedInUser,
} from 'teleterm/services/tshd/testHelpers';

import { hasConnectMyComputerPermissions } from './permissions';

const testCases: {
  name: string;
  platform: Platform;
  canCreateToken: boolean;
  expect: boolean;
}[] = [
  {
    name: 'should be true when OS is darwin and can create token',
    platform: 'darwin',
    canCreateToken: true,
    expect: true,
  },
  {
    name: 'should be true when OS is  linux and can create token',
    platform: 'linux',
    canCreateToken: true,
    expect: true,
  },
  {
    name: 'should be false when OS is windows and can create token',
    platform: 'win32',
    canCreateToken: true,
    expect: false,
  },
  {
    name: 'should be false when OS is darwin and cannot create token',
    platform: 'darwin',
    canCreateToken: false,
    expect: false,
  },
];

test.each(testCases)('$name', testCase => {
  const cluster = makeRootCluster({
    loggedInUser: makeLoggedInUser({
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
    }),
  });
  const runtimeSettings = makeRuntimeSettings({ platform: testCase.platform });

  const isPermitted = hasConnectMyComputerPermissions(cluster, runtimeSettings);
  expect(isPermitted).toEqual(testCase.expect);
});
