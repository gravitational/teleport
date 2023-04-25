import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import {
  AddCardRequest,
  BillingCycle,
  BillingInformation,
  Invoice,
  RemoveCardRequest,
  SetupIntent,
  UpdateAccountRequest,
  UpdateCardRequest,
} from './types';

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
    return api.get(cfg.api.billingPath).then(makeBillingInformation);
  }

  fetchInvoices(): Promise<Invoice[]> {
    return api.get(cfg.api.invoicesPath).then((json: Invoice[]) => json || []);
  }

  fetchBillingCycles(): Promise<BillingCycle[]> {
    return api.get(cfg.api.cyclesPath).then(makeBillingCycles);
  }

  createSetupIntent(): Promise<SetupIntent> {
    return api.post(cfg.api.setupIntentPath).then(makeSetupIntentResponse);
  }
}

export default CloudService;

function makeBillingCycles(json: any) {
  json = json || [];

  return json.map((cycle: BillingCycle) => ({
    ...cycle,
    itemsList: cycle.itemsList || [],
  })) as BillingCycle[];
}

function makeBillingInformation(json: any) {
  json.cardsList = json.cardsList || [];

  return json as BillingInformation;
}

function makeSetupIntentResponse(json: any) {
  json.clientSecret = json.clientSecret || '';

  return json as SetupIntent;
}
