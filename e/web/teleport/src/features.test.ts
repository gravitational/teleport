import { Context } from 'teleport';
import { userContext, fullAcl } from 'teleport/Main/fixtures';
import TeleportContextE from 'e-teleport/teleportContextE';
import getFeaturesE from 'e-teleport/features';
import getFeaturesOss from 'teleport/features';

test('enterprise features are a superset of oss features', () => {
  // Ensure userContext has full permissions.
  userContext.acl = fullAcl;

  // Create an OSS and Enterprise context with identical userContext.
  const ctxOss = new Context();
  ctxOss.isEnterprise = false;
  ctxOss.storeUser.setState(userContext);

  const ctxE = new TeleportContextE();
  ctxE.isEnterprise = true;
  ctxE.storeUser.setState(userContext);

  // Run getFeatures with corresponding context.
  getFeaturesOss().forEach(f => f.register(ctxOss));
  getFeaturesE().forEach(f => f.register(ctxE));

  // For each feature in OSS, check that there is an equivalent in Enterprise
  ctxOss.features.forEach(featureOss => {
    expect(
      ctxE.features.find(
        featureE => featureE.route.title === featureOss.route.title
      )
    ).toBeDefined();
  });
});
