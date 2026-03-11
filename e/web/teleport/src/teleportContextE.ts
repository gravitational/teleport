import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import { ACCESS_GRAPH_JS_URL } from 'e-teleport/AccessGraph/loader';
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
import { clientIpRestrictionsService } from './services/clientiprestrictions';
import { contactsService } from './services/contacts';
import { downloadsService } from './services/downloads';
import { externalAuditStorageService } from './services/externalauditstorage';
import { IdpService } from './services/idp';
import { pluginsService } from './services/plugins';
import { upgradeWindowService } from './services/upgradeWindow';

class TeleportEContext extends TeleportContext {
  // stores
  storeAccessRequests = new StoreAccessRequests();

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
  contactService = contactsService;
  clientIpRestrictionsService = clientIpRestrictionsService;

  notificationContentFactory = notificationContentFactoryE;

  // init fetches data required for initial rendering of components.
  // The caller of this function provides the try/catch
  // block.
  async init(preferences: UserPreferences) {
    await super.init(preferences);

    // If access graph is enabled, prefetch the JS bundle for faster launching.
    if (storageService.getAccessGraphEnabled()) {
      const link = document.createElement('link');
      link.rel = 'prefetch';
      link.href = ACCESS_GRAPH_JS_URL;
      document.head.appendChild(link);
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
