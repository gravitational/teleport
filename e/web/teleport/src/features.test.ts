import { getEnterpriseFeatures } from 'e-teleport/features';
import cfg from 'teleport/config';
import { canShowFeature, getOSSFeatures } from 'teleport/features';
import { disabledFeatureFlags } from 'teleport/teleportContext';
import { NavTitle } from 'teleport/types';

const enterpriseTitles: string[] = getEnterpriseFeatures()
  // some features do not have a route defined, so we need to filter them out
  .filter(feature => Boolean(feature.route))
  .map(f => f.route.title);

const ossTitles: string[] = getOSSFeatures()
  // some features do not have a route defined, so we need to filter them out
  .filter(feature => Boolean(feature.route))
  .map(f => f.route.title);

test.each(ossTitles)(`oss title %s should exist in Enterprise`, testCase => {
  // AccessRequests is a non-feature view in OSS to show off Enterprise capability
  // An actual working route exists in Enterprise
  if (testCase === NavTitle.AccessRequests) {
    return;
  }

  expect(enterpriseTitles).toContain(testCase);
});

let originalEntitlements: typeof cfg.entitlements;

beforeEach(() => {
  originalEntitlements = structuredClone(cfg.entitlements);
});

afterEach(() => {
  cfg.entitlements = originalEntitlements;
});

// A user with no permissions should see no navigation entries other than Resources when feature hiding is on.
test('a user without any access sees no navigation items', () => {
  // Enable every entitlement, otherwise features that depend on one wouldn't be caught by this test
  for (const entitlement of Object.values(cfg.entitlements)) {
    entitlement.enabled = true;
  }

  const visible = getEnterpriseFeatures()
    .filter(feature => canShowFeature(feature, disabledFeatureFlags))
    .filter(feature => !!feature.navigationItem)
    .map(feature => feature.navigationItem.title);

  expect(visible).toEqual(['Resources']);
});
