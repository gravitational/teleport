import api from 'teleport/services/api';
import {
  UpdateAccountRequest,
  Invoice,
  BillingInformation,
  BillingCycle,
  AddCardRequest,
  UpdateCardRequest,
  RemoveCardRequest,
} from './types';
import cfg from 'e-teleport/config';

class CloudService {
  updateAccount(req: UpdateAccountRequest) {
    return api.put(cfg.api.accountPath, req);
  }

  addCard(req: AddCardRequest) {
    return api.post(cfg.api.cardPath, req);
  }

  removeCard(req: RemoveCardRequest) {
    return api.delete(cfg.api.cardPath, req);
  }

  updateCard(req: UpdateCardRequest) {
    return api.put(cfg.api.cardPath, req);
  }

  fetchBillingInformation(): Promise<BillingInformation> {
    return api.get(cfg.api.billingPath);
  }

  fetchInvoices() {
    return api.get(cfg.api.invoicesPath).then((json: Invoice[]) => json || []);
  }

  fetchBillingCycles() {
    return api
      .get(cfg.api.cyclesPath)
      .then((json: BillingCycle[]) => json || []);
  }
}

export default CloudService;
