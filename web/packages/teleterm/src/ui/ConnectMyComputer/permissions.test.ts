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

import { createMockConfigService } from 'teleterm/services/config/fixtures/mocks';
import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';
import { Platform } from 'teleterm/mainProcess/types';
import {
  makeRootCluster,
  makeLoggedInUser,
} from 'teleterm/services/tshd/testHelpers';

import { canUseConnectMyComputer } from './permissions';

const testCases: {
  name: string;
  platform: Platform;
  canCreateToken: boolean;
  isFeatureFlagEnabled: boolean;
  isAgentConfigured: boolean;
  expect: boolean;
}[] = [
  {
    name: 'darwin, can create token, feature flag enabled, agent not configured',
    platform: 'darwin',
    canCreateToken: true,
    isFeatureFlagEnabled: true,
    isAgentConfigured: false,
    expect: true,
  },
  {
    name: 'linux, can create token, feature flag enabled, agent not configured',
    platform: 'linux',
    canCreateToken: true,
    isFeatureFlagEnabled: true,
    isAgentConfigured: false,
    expect: true,
  },
  {
    name: 'windows, can create token, feature flag enabled, agent not configured',
    platform: 'win32',
    canCreateToken: true,
    isFeatureFlagEnabled: true,
    isAgentConfigured: false,
    expect: false,
  },
  {
    name: 'darwin, cannot create token, feature flag enabled, agent not configured',
    platform: 'darwin',
    canCreateToken: false,
    isFeatureFlagEnabled: true,
    isAgentConfigured: false,
    expect: false,
  },
  {
    name: 'darwin, can create token, feature flag not enabled, agent not configured',
    platform: 'darwin',
    canCreateToken: true,
    isFeatureFlagEnabled: false,
    isAgentConfigured: false,
    expect: false,
  },
  {
    name: 'darwin, cannot create token, feature flag enabled, agent configured',
    platform: 'darwin',
    canCreateToken: false,
    isFeatureFlagEnabled: true,
    isAgentConfigured: true,
    expect: true,
  },
  {
    name: 'darwin, cannot create token, feature flag not enabled, agent configured',
    platform: 'darwin',
    canCreateToken: false,
    isFeatureFlagEnabled: false,
    isAgentConfigured: true,
    expect: false,
  },
  {
    name: 'windows, cannot create token, feature flag enabled, agent configured',
    platform: 'win32',
    canCreateToken: false,
    isFeatureFlagEnabled: true,
    isAgentConfigured: true,
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
  const configService = createMockConfigService({
    'feature.connectMyComputer': testCase.isFeatureFlagEnabled,
  });
  const runtimeSettings = makeRuntimeSettings({ platform: testCase.platform });

  const isPermitted = canUseConnectMyComputer(
    cluster,
    configService,
    runtimeSettings,
    testCase.isAgentConfigured
  );
  expect(isPermitted).toEqual(testCase.expect);
});
