import TeleportContext from 'teleport/teleportContext';
import WorkflowService from 'e-teleport/services/workflow';
import ResourceService from 'e-teleport/services/resource';
import StoreAccessRequests from 'e-teleport/stores/storeAccessRequests';
import CloudService from 'e-teleport/services/cloud';
import RecoveryService from 'e-teleport/services/recovery';

class TeleportEContext extends TeleportContext {
  storeAccessRequests = new StoreAccessRequests();
  workflowService = new WorkflowService();
  resourceService = new ResourceService();
  cloudService = new CloudService();
  recoveryService = new RecoveryService();
}

export default TeleportEContext;
