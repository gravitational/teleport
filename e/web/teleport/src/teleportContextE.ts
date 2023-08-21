import TeleportContext from 'teleport/teleportContext';

import localStorage from 'teleport/services/localStorage';

import * as service from 'teleport/services/userPreferences';

import cfg from 'teleport/config';

import WorkflowService from 'e-teleport/services/workflow';
import ResourceService from 'e-teleport/services/resource';
import StoreAccessRequests from 'e-teleport/stores/storeAccessRequests';
import CloudService from 'e-teleport/services/cloud';
import RecoveryService from 'e-teleport/services/recovery';

import { deviceService } from 'e-teleport/services/devices';

import { surveyService } from 'e-teleport/services/survey';

import { setAndEmitFeatureRecommendationStatus } from 'e-teleport/services/featureRecommendation';

import { downloadsService } from './services/downloads';
import { pluginsService } from './services/plugins';

import { upgradeWindowService } from './services/upgradeWindow';
import { IdpService } from './services/idp';

class TeleportEContext extends TeleportContext {
  storeAccessRequests = new StoreAccessRequests();
  workflowService = new WorkflowService();
  resourceService = new ResourceService();
  cloudService = new CloudService();
  recoveryService = new RecoveryService();
  upgradeWindowService = upgradeWindowService;
  downloadsService = downloadsService;
  pluginsService = pluginsService;
  deviceService = deviceService;
  idpService = new IdpService();

  // init fetches data required for initial rendering of components.
  // The caller of this function provides the try/catch
  // block.
  async init() {
    await super.init();
    const survey = localStorage.getOnboardSurvey();
    if (survey) {
      const { clusterResources, ...rest } = survey;

      // submit survey to sales center
      surveyService.submitSurvey(rest);

      if (clusterResources && clusterResources.length > 0) {
        // add survey resources to cluster state
        await service.updateUserPreferences({
          onboard: {
            preferredResources: clusterResources,
          },
        });
      }
      localStorage.clearOnboardSurvey();
    }

    // fetchNonBillableSummaryInformation will do an auth check on the backend for the billing role,
    // we should only fetch if the user has the correct permissions.
    if (cfg.isUsageBasedBilling && this.getFeatureFlags().billing) {
      try {
        await setAndEmitFeatureRecommendationStatus(this.cloudService);
      } catch (err) {
        // log error instead of bubbling it up and crashing the app
        console.error(err);
      }
    }
  }
}

export default TeleportEContext;
