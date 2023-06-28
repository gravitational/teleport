import { createMockConfigService } from 'teleterm/services/config/fixtures/mocks';
import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';
import { Platform } from 'teleterm/mainProcess/types';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import { canUseConnectMyComputer } from './permissions';

const testCases: {
  name: string;
  platform: Platform;
  canCreateToken: boolean;
  isUsageBasedBilling: boolean;
  isFeatureFlagEnabled: boolean;
  expect: boolean;
}[] = [
  {
    name: 'darwin, can create token, usage based plan, feature flag enabled',
    platform: 'darwin',
    canCreateToken: true,
    isUsageBasedBilling: true,
    isFeatureFlagEnabled: true,
    expect: true,
  },
  {
    name: 'linux, can create token, usage based plan, feature flag enabled',
    platform: 'linux',
    canCreateToken: true,
    isUsageBasedBilling: true,
    isFeatureFlagEnabled: true,
    expect: true,
  },
  {
    name: 'windows, can create token, usage based plan, feature flag enabled',
    platform: 'win32',
    canCreateToken: true,
    isUsageBasedBilling: true,
    isFeatureFlagEnabled: true,
    expect: false,
  },
  {
    name: 'darwin, cannot create token, usage based plan, feature flag enabled',
    platform: 'darwin',
    canCreateToken: false,
    isUsageBasedBilling: true,
    isFeatureFlagEnabled: true,
    expect: false,
  },
  {
    name: 'darwin, can create token, non-usage based plan, feature flag enabled',
    platform: 'darwin',
    canCreateToken: true,
    isUsageBasedBilling: false,
    isFeatureFlagEnabled: true,
    expect: false,
  },
  {
    name: 'darwin, can create token, usage based plan, feature flag not enabled',
    platform: 'darwin',
    canCreateToken: true,
    isUsageBasedBilling: true,
    isFeatureFlagEnabled: false,
    expect: false,
  },
];

test.each(testCases)('$name', testCase => {
  const cluster = makeRootCluster({
    features: {
      advancedAccessWorkflows: false,
      isUsageBasedBilling: testCase.isUsageBasedBilling,
    },
    loggedInUser: {
      name: 'test',
      activeRequestsList: [],
      assumedRequests: {},
      rolesList: [],
      sshLoginsList: [],
      requestableRolesList: [],
      suggestedReviewersList: [],
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
    },
  });
  const configService = createMockConfigService({
    'feature.connectMyComputer': testCase.isFeatureFlagEnabled,
  });
  const runtimeSettings = makeRuntimeSettings({ platform: testCase.platform });

  const isPermitted = canUseConnectMyComputer(
    cluster,
    configService,
    runtimeSettings
  );
  expect(isPermitted).toEqual(testCase.expect);
});
