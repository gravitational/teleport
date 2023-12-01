import { getErrMessage } from 'shared/utils/errorType';
import TeleportContext from 'teleport/teleportContext';
import { storageService } from 'teleport/services/storageService';
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
import { accessManagementService } from './services/accessmanagement';
import { StoreNotificationsE } from './stores/storeNotificationsE';
import { externalAuditStorageService } from './services/externalauditstorage';

class TeleportEContext extends TeleportContext {
  // stores
  storeAccessRequests = new StoreAccessRequests();
  storeNotifications = new StoreNotificationsE();

  // services
  workflowService = new WorkflowService();
  resourceService = new ResourceService();
  cloudService = new CloudService();
  recoveryService = new RecoveryService();
  upgradeWindowService = upgradeWindowService;
  downloadsService = downloadsService;
  pluginsService = pluginsService;
  deviceService = deviceService;
  idpService = new IdpService();
  externalAuditStorageService = externalAuditStorageService;

  // init fetches data required for initial rendering of components.
  // The caller of this function provides the try/catch
  // block.
  async init() {
    await super.init();

    try {
      const accessLists = await accessManagementService.fetchAccessLists();
      this.storeNotifications.setNotificationsForAccessListsRequiringReview(
        accessLists,
        this.storeUser.state
      );
    } catch (err) {
      // An error is most likely from access denied, so we'll
      // ignore it.
      //
      // An access list can only be fetched if this user is either
      // admin (rbac) or is an owner or member of access lists,
      // otherwise returns an error.
      //
      // There could be a possiblilty that the error is not a type of
      // access denied (eg: network blip), but chose to ignore it anyways
      // for the following reasons:
      //   1) upon refreshing the browser, the app reboots, so fetching
      //      access lists will be attempted again
      //   2) when a user visits the page for listing access lists,
      //      the notifications for access list will be updated with
      //      the access lists that were fetched for this page
      //      (b/c it's fresher data)
      //   3) we give users two weeks advance notice for due dates,
      //      which should give users plenty of chances to get this notice
      console.warn('Failed to set notifications: ', getErrMessage(err));
    }

    const survey = storageService.getOnboardSurvey();
    if (survey) {
      const { clusterResources, marketingParams, ...rest } = survey;

      // submit survey to sales center
      surveyService.submitSurvey(rest);

      if (clusterResources && clusterResources.length > 0) {
        // add survey resources to cluster state
        await service.updateUserPreferences({
          onboard: {
            preferredResources: clusterResources,
            marketingParams: marketingParams,
          },
        });
      }
      storageService.clearOnboardSurvey();
    }

    // fetchNonBillableSummaryInformation will do an auth check on the backend for the billing role,
    // we should only fetch if the user has the correct permissions.
    if (cfg.isTeam && this.getFeatureFlags().billing) {
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
