import { getOSSFeatures } from 'teleport/features';

import { getEnterpriseFeatures } from 'e-teleport/features';

test('enterprise features are a superset of oss features', () => {
  // some features do not have a route defined, so we need to filter them out
  const featuresE = getEnterpriseFeatures().filter(featureE =>
    Boolean(featureE.route)
  );
  const featuresOSS = getOSSFeatures().filter(feature =>
    Boolean(feature.route)
  );

  // For each feature in OSS, check that there is an equivalent in Enterprise
  featuresOSS.forEach(featureOss => {
    if (
      !featuresE.find(
        featureE => featureE.route.title === featureOss.route.title
      )
    ) {
      console.log(featureOss);
    }
    expect(
      featuresE.find(
        featureE => featureE.route.title === featureOss.route.title
      )
    ).toBeDefined();
  });
});
