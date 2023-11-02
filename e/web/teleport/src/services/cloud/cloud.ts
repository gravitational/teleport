import api from 'teleport/services/api';
import { User } from 'teleport/services/user/types';

import cfg from 'e-teleport/config';

import {
  AddCardRequest,
  BillingInformation,
  BillingSummaryInformation,
  InvoiceSettingsInformation,
  PaymentsInvoicesInformation,
  RemoveCardRequest,
  SetupIntent,
  StripeBillingAddressRequest,
  UpdateCardRequest,
  UpdateEmailRequest,
  UpdatePurchaseOrderRequest,
  NonBillableSummaryInformation,
  SendTeleportInvite,
} from './types';

class CloudService {
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

  fetchNonBillableSummaryInformation(): Promise<NonBillableSummaryInformation> {
    return api
      .get(cfg.api.nonBillableUsageSummaryPath)
      .then(makeNonBillableUsageSummary);
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

  updateEmail(req: UpdateEmailRequest) {
    return api.put(cfg.api.emailPath, req);
  }

  updatePurchaseOrderPrefix(req: UpdatePurchaseOrderRequest) {
    return api.put(cfg.api.poPath, req);
  }

  cancelSubscription() {
    return api.delete(cfg.api.billingPath);
  }

  sendTeleportInvite(req: SendTeleportInvite): Promise<User[]> {
    return api.post(cfg.api.teleportInvitePath, req);
  }
}

export default CloudService;

function makeBillingInformation(json: any) {
  json.cardsList = json.cardsList || [];

  return json as BillingInformation;
}

function makeBillingSummaryInformation(json: any) {
  return json as BillingSummaryInformation;
}

function makeNonBillableUsageSummary(json: any) {
  return json as NonBillableSummaryInformation;
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
