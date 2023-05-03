import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import {
  AddCardRequest,
  BillingCycle,
  BillingInformation,
  BillingSummaryInformation,
  Invoice,
  InvoiceSettingsInformation,
  PaymentsInvoicesInformation,
  RemoveCardRequest,
  SetupIntent,
  UpdateAccountRequest,
  StripeBillingAddressRequest,
  UpdateCardRequest,
} from './types';

class CloudService {
  updateAccount(req: UpdateAccountRequest) {
    return api.put(cfg.api.accountPath, req);
  }

  updateAddress(req: StripeBillingAddressRequest) {
    return api.put(cfg.api.addressPath, req);
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

  fetchInvoices(): Promise<Invoice[]> {
    return api.get(cfg.api.invoicesPath).then((json: Invoice[]) => json || []);
  }

  fetchBillingCycles(): Promise<BillingCycle[]> {
    return api.get(cfg.api.cyclesPath).then(makeBillingCycles);
  }

  createSetupIntent(): Promise<SetupIntent> {
    return api.post(cfg.api.setupIntentPath).then(makeSetupIntentResponse);
  }

  fetchBillingInformation(): Promise<BillingInformation> {
    return api.get(cfg.api.billingPath).then(makeBillingInformation);
  }

  fetchBillingSummaryInformation(): Promise<BillingSummaryInformation> {
    return api
      .get(cfg.api.billingSummaryPath)
      .then(makeBillingSummaryInformation);
  }

  fetchPaymentsAndInvoices(): Promise<PaymentsInvoicesInformation> {
    return api
      .get(cfg.api.paymentsInvoicesPath)
      .then(makePaymentsInvoicesInformation);
  }

  fetchInvoiceSettings(): Promise<InvoiceSettingsInformation> {
    return api
      .get(cfg.api.invoiceSettingsPath)
      .then(makeInvoiceSettingsInformation);
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

function makeBillingSummaryInformation(json: any) {
  return json as BillingSummaryInformation;
}

function makePaymentsInvoicesInformation(json: any) {
  json.stripeInvoicesList = json.stripeInvoices || [];
  json.stripeCardsList = json.stripeCards || [];

  return json as PaymentsInvoicesInformation;
}

function makeInvoiceSettingsInformation(json: any) {
  return json as InvoiceSettingsInformation;
}

function makeSetupIntentResponse(json: any) {
  json.clientSecret = json.clientSecret || '';

  return json as SetupIntent;
}
