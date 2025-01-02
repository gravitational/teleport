import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';
import { getErrMessage } from 'shared/utils/errorType';

import eCfg from 'e-teleport/config';
import CloudService from 'e-teleport/services/cloud';
import { deviceService } from 'e-teleport/services/devices';
import RecoveryService from 'e-teleport/services/recovery';
import ResourceService from 'e-teleport/services/resource';
import { surveyService } from 'e-teleport/services/survey';
import WorkflowService from 'e-teleport/services/workflow';
import StoreAccessRequests from 'e-teleport/stores/storeAccessRequests';
import cfg from 'teleport/config';
import { storageService } from 'teleport/services/storageService';
import * as service from 'teleport/services/userPreferences';
import TeleportContext from 'teleport/teleportContext';

import { notificationContentFactoryE } from './Notifications';
import { accessManagementService } from './services/accessmanagement';
import { contactsService } from './services/contacts';
import { downloadsService } from './services/downloads';
import { externalAuditStorageService } from './services/externalauditstorage';
import { IdpService } from './services/idp';
import { pluginsService } from './services/plugins';
import { upgradeWindowService } from './services/upgradeWindow';
import { StoreNotificationsE } from './stores/storeNotificationsE';

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
  redirectUrl: string | null = null;
  contactService = contactsService;

  notificationContentFactory = notificationContentFactoryE;

  // init fetches data required for initial rendering of components.
  // The caller of this function provides the try/catch
  // block.
  async init(preferences: UserPreferences) {
    await super.init(preferences);

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

    if (
      cfg.isCloud &&
      (!preferences.accessGraph || !preferences.accessGraph.hasBeenRedirected)
    ) {
      try {
        // the marketing UTM params are only retrievable from the survey service
        const survey = await surveyService.getSurveyCompanyResults();

        if (survey.marketingParams.intent === 'policy') {
          await service.updateUserPreferences({
            accessGraph: {
              hasBeenRedirected: true,
            },
          });

          this.redirectUrl = eCfg.routes.accessGraph.dashboard;
        }
      } catch {
        // it's okay if we can't fetch the marketing params
      }
    }

    const survey = storageService.getOnboardSurvey();
    if (survey) {
      const { clusterResources, marketingParams, ...rest } = survey;

      // submit survey to sales center
      surveyService.submitSurvey({
        companyName: rest.companyName,
        employeeCount: rest.employeeCount,
        resources: rest.resources,
        role: rest.role,
        team: rest.team,
      });

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

    // check if there's already an external audit storage configured
    // so that we don't show the feature's CTAs
    const isDismissed = storageService.getExternalAuditStorageCtaDisabled();
    if (
      !isDismissed &&
      this.isCloud &&
      cfg.externalAuditStorage &&
      this.storeUser.getExternalAuditStorageAccess().read
    ) {
      try {
        const externalAuditStorage =
          await this.externalAuditStorageService.getCluster();
        if (externalAuditStorage) {
          this.hasExternalAuditStorage = true;
        }
      } catch (err) {
        // log error instead of bubbling it up and crashing the app
        console.error(err);
      }
    }
  }
}

export default TeleportEContext;
