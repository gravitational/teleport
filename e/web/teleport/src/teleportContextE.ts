import TeleportContext from 'teleport/teleportContext';
import WorkflowService from 'e-teleport/services/workflow';
import StoreAccessRequests from 'e-teleport/stores/storeAccessRequests';

class TeleportEContext extends TeleportContext {
  storeAccessRequests = new StoreAccessRequests();
  workflowService = new WorkflowService();
}

export default TeleportEContext;
