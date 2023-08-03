import TeleportContext from 'teleport/teleportContext';

import localStorage from 'teleport/services/localStorage';

import * as service from 'teleport/services/userPreferences';

import { RecommendationStatus } from 'teleport/types';

import cfg from 'teleport/config';

import WorkflowService from 'e-teleport/services/workflow';
import ResourceService from 'e-teleport/services/resource';
import StoreAccessRequests from 'e-teleport/stores/storeAccessRequests';
import CloudService from 'e-teleport/services/cloud';
import RecoveryService from 'e-teleport/services/recovery';

import { deviceService } from 'e-teleport/services/devices';

import { surveyService } from 'e-teleport/services/survey';

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

    if (cfg.isUsageBasedBilling) {
      // retrieve feature recommendation state from local storage.
      // if grv_recommend_feature is undefined, or a given feature recommendation state is
      // not 'DONE', we check with backend for usage summary and set state to 'NOTIFY'
      // if usage counts zero.
      // if usage counts more than zero, we set state to 'DONE'.
      const recommendFeature = localStorage.getFeatureRecommendationStatus();

      if (
        !recommendFeature ||
        recommendFeature?.TrustedDevices !== RecommendationStatus.Done
      ) {
        // TODO(sshah): update to status 'DONE' once user completes desired CTA.
        const nonBillableUsage =
          await this.cloudService.fetchNonBillableSummaryInformation();

        const trustedDevicesNotificationStatus = nonBillableUsage
          .trustedDeviceUsage.devicesInUse
          ? RecommendationStatus.Done
          : RecommendationStatus.Notify;
        localStorage.setRecommendFeature({
          TrustedDevices: trustedDevicesNotificationStatus,
        });
      }
    }
  }
}

export default TeleportEContext;
