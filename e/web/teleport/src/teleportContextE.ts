import TeleportContext from 'teleport/teleportContext';

import WorkflowService from 'e-teleport/services/workflow';
import ResourceService from 'e-teleport/services/resource';
import StoreAccessRequests from 'e-teleport/stores/storeAccessRequests';
import CloudService from 'e-teleport/services/cloud';
import RecoveryService from 'e-teleport/services/recovery';

import { deviceService } from 'e-teleport/services/devices';

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
}

export default TeleportEContext;
