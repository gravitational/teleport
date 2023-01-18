import { getOSSFeatures } from 'teleport/features';

import { getEnterpriseFeatures } from 'e-teleport/features';

test('enterprise features are a superset of oss features', () => {
  const featuresE = getEnterpriseFeatures();

  // For each feature in OSS, check that there is an equivalent in Enterprise
  getOSSFeatures().forEach(featureOss => {
    expect(
      featuresE.find(
        featureE => featureE.route.title === featureOss.route.title
      )
    ).toBeDefined();
  });
});
