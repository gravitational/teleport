import { getEnterpriseFeatures } from 'e-teleport/features';
import { getOSSFeatures } from 'teleport/features';
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
