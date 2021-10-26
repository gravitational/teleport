import getFeaturesE from 'e-teleport/features';
import getFeaturesOss from 'teleport/features';

test('enterprise features are a superset of oss features', () => {
  const featuresE = getFeaturesE();

  // For each feature in OSS, check that there is an equivalent in Enterprise
  getFeaturesOss().forEach(featureOss => {
    expect(
      featuresE.find(
        featureE => featureE.route.title === featureOss.route.title
      )
    ).toBeDefined();
  });
});
