import cfg from 'e-teleport/config';
import {
  GetMAUDailyBreakdownRequest,
  GetMAUDailyBreakdownResponse,
  GetStripeConfigResponse,
  GetTPRDailyBreakdownRequest,
  GetTPRDailyBreakdownResponse,
  GetAccountUpgradeWindowStartHourResponse,
  GetEnvironmentProfileResponse,
  GetUsageRequest,
  GetUsageResponse,
  MAUDailyPoint,
  StripeCancelResponse,
  StripeCard,
  StripeCreateCardRequest,
  StripeCreateCardResponse,
  StripeCreateSetupIntentResponse,
  StripeDeleteCardRequest,
  StripeDeleteCardResponse,
  StripeGetSettingsResponse,
  StripeInvoice,
  StripeListCardsResponse,
  StripeListInvoicesResponse,
  StripeUpdateCardRequest,
  StripeUpdateCardResponse,
  StripeUpdateEmailRequest,
  StripeUpdateEmailResponse,
  StripeUpdatePOPrefixRequest,
  StripeUpdatePOPrefixResponse,
  StripeUpdateStripeAddressRequest,
  StripeUpdateStripeAddressResponse,
  TPRDailyPoint,
  UpdateAccountUpgradeWindowStartHourRequest,
  UpdateEnvironmentProfileRequest,
  Usage,
  UsageCycle,
  UsageLimits,
} from 'e-teleport/services/cloud/v1/tenants_pb';
import api from 'teleport/services/api';
import auth from 'teleport/services/auth/auth';
import { User } from 'teleport/services/user/types';

import {
  NonBillableSummaryInformation,
  SendTeleportCredentialReset,
  SendTeleportInvite,
} from './types';

export const availableEnvironmentProfiles = ['production', 'staging'];
export const availableUpgradeWindowStartHours = [8, 16, 23] as const;
export type UpgradeWindowStartHour =
  (typeof availableUpgradeWindowStartHours)[number];
export type EnvironmentProfile = 'production' | 'staging';

class CloudService {
  fetchBillingSummaryInformation(
    req: GetUsageRequest
  ): Promise<GetUsageResponse> {
    return api.post(cfg.api.billingSummaryPath, req).then(makeGetUsageResponse);
  }

  fetchNonBillableSummaryInformation(): Promise<NonBillableSummaryInformation> {
    return api
      .get(cfg.api.nonBillableUsageSummaryPath)
      .then(makeNonBillableUsageSummary);
  }

  async sendTeleportInvite(req: SendTeleportInvite): Promise<User[]> {
    const mfaResponse = await auth.getMfaChallengeResponseForAdminAction(true);
    return api.post(cfg.api.teleportInvitePath, req, null, mfaResponse);
  }

  async sendTeleportCredentialReset(
    req: SendTeleportCredentialReset
  ): Promise<void> {
    const mfaResponse = await auth.getMfaChallengeResponseForAdminAction(true);
    return api.post(
      cfg.api.teleportCredentialResetPath,
      req,
      null,
      mfaResponse
    );
  }

  getUpgradeWindowStartHour(clusterId): Promise<UpgradeWindowStartHour> {
    return api
      .get(cfg.getWindowUpgradeStartUrl(clusterId))
      .then(
        (res: GetAccountUpgradeWindowStartHourResponse) =>
          res.upgradeWindowStartHour as UpgradeWindowStartHour
      );
  }

  updateUpgradeWindowStart(
    clusterId,
    upgradeWindowStartHour: UpgradeWindowStartHour
  ): Promise<UpgradeWindowStartHour> {
    const req: UpdateAccountUpgradeWindowStartHourRequest = {
      upgradeWindowStartHour,
    };
    return api
      .post(cfg.getWindowUpgradeStartUrl(clusterId), req)
      .then(
        (res: GetAccountUpgradeWindowStartHourResponse) =>
          res.upgradeWindowStartHour as UpgradeWindowStartHour
      );
  }

  getEnvironmentProfile(): Promise<GetEnvironmentProfileResponse> {
    return api.get(cfg.api.environmentProfileUrl);
  }

  updateEnvironmentProfile(
    profile: EnvironmentProfile
  ): Promise<GetEnvironmentProfileResponse> {
    const req: UpdateEnvironmentProfileRequest = {
      environmentProfile: profile,
    };
    return api.post(cfg.api.environmentProfileUrl, req);
  }

  fetchMAUDailyBreakdown(
    req: GetMAUDailyBreakdownRequest
  ): Promise<GetMAUDailyBreakdownResponse> {
    const params = buildBreakdownQueryParams(req);
    return api
      .get(`${cfg.api.mauBreakdownPath}?${params}`)
      .then(makeMAUDailyBreakdownResponse);
  }

  fetchTPRDailyBreakdown(
    req: GetTPRDailyBreakdownRequest
  ): Promise<GetTPRDailyBreakdownResponse> {
    const params = buildBreakdownQueryParams(req);
    return api
      .get(`${cfg.api.tprBreakdownPath}?${params}`)
      .then(makeTPRDailyBreakdownResponse);
  }

  fetchStripeConfig(): Promise<GetStripeConfigResponse> {
    return api.get(cfg.api.stripeConfigPath).then(makeStripeConfigResponse);
  }

  fetchCards(): Promise<StripeListCardsResponse> {
    return api.get(cfg.api.stripeCardsPath).then(makeStripeListCardsResponse);
  }

  createSetupIntent(): Promise<StripeCreateSetupIntentResponse> {
    return api
      .post(cfg.api.stripeSetupIntentPath, {})
      .then(makeStripeCreateSetupIntentResponse);
  }

  createCard(req: StripeCreateCardRequest): Promise<StripeCreateCardResponse> {
    return api.post(cfg.api.stripeCardsPath, req).then(() => ({}));
  }

  updateCard(req: StripeUpdateCardRequest): Promise<StripeUpdateCardResponse> {
    return api.put(cfg.api.stripeCardsPath, req).then(() => ({}));
  }

  deleteCard(req: StripeDeleteCardRequest): Promise<StripeDeleteCardResponse> {
    return api
      .deleteWithOptions(cfg.api.stripeCardsPath, {
        data: req,
        headers: { 'Content-Type': 'application/json' },
      })
      .then(() => ({}));
  }

  fetchInvoices(): Promise<StripeListInvoicesResponse> {
    return api
      .get(cfg.api.stripeInvoicesPath)
      .then(makeStripeListInvoicesResponse);
  }

  fetchStripeSettings(): Promise<StripeGetSettingsResponse> {
    return api
      .get(cfg.api.stripeInvoiceSettingsPath)
      .then(makeStripeGetSettingsResponse);
  }

  updateInvoiceEmail(
    req: StripeUpdateEmailRequest
  ): Promise<StripeUpdateEmailResponse> {
    return api.post(cfg.api.stripeInvoiceEmailPath, req).then(() => ({}));
  }

  updatePurchaseOrderPrefix(
    req: StripeUpdatePOPrefixRequest
  ): Promise<StripeUpdatePOPrefixResponse> {
    return api.post(cfg.api.stripeInvoicePOPrefixPath, req).then(() => ({}));
  }

  updateStripeAddress(
    req: StripeUpdateStripeAddressRequest
  ): Promise<StripeUpdateStripeAddressResponse> {
    return api.post(cfg.api.stripeInvoiceAddressPath, req).then(() => ({}));
  }

  cancelSubscription(): Promise<StripeCancelResponse> {
    return api.post(cfg.api.stripeCancelPath, {}).then(() => ({}));
  }
}

export default CloudService;

function makeGetUsageResponse(json: any): GetUsageResponse {
  return {
    alerts: json?.alerts || [],
    usageHistory: makeUsageHistory(json?.usageHistory || []),
    missingEntitlements: json?.missingEntitlements || [],
    aggregateCount: json?.aggregateCount || 0,
    usageUpdatedAt: json?.usageUpdatedAt || 0,
  };
}

function makeUsageHistory(json: any): UsageCycle[] {
  return json.map(j => {
    return {
      usage: makeUsage(j.usage || {}),
      usageLimits: makeUsageLimits(j.usageLimits || {}),
      start: j.start || 0,
      startFormatted: j.startFormatted || '',
      end: j.end || 0,
      endFormatted: j.endFormatted || '',
      activeAccounts: j.activeAccounts || 0,
      calibratingAccounts: j.calibratingAccounts || 0,
      pricingModel: {
        version: j?.pricingModel.version,
        modelId: j?.pricingModel.modelId,
        name: j?.pricingModel.name,
        createdAt: j?.pricingModel.createdAt,
        description: j?.pricingModel.description,
        metric: j?.pricingModel.metric,
      },
    };
  });
}

function makeUsage(json: any): Usage {
  return {
    igmau: json?.igmau || 0,
    mwi: json?.mwi || 0,
    tpr: json?.tpr || 0,
    ztamau: json?.ztamau || 0,
  };
}

function makeUsageLimits(json: any): UsageLimits {
  return {
    igmau: json?.igmau || 0,
    mwi: json?.mwi || 0,
    tpr: json?.tpr || 0,
    ztamau: json?.ztamau || 0,
  };
}

function makeNonBillableUsageSummary(json: any) {
  return json as NonBillableSummaryInformation;
}

function makeMAUDailyBreakdownResponse(
  json: any
): GetMAUDailyBreakdownResponse {
  return {
    days: (json?.days || []).map(makeMAUDailyPoint),
    rangeStart: Number(json?.rangeStart) || 0,
    rangeEnd: Number(json?.rangeEnd) || 0,
    calibratingAccounts: json.calibratingAccounts || 0,
  };
}

function makeMAUDailyPoint(json: any): MAUDailyPoint {
  return {
    day: Number(json?.day) || 0,
    newInWindow: Number(json?.newInWindow) || 0,
    returning: Number(json?.returning) || 0,
    contributingClusters: json?.contributingClusters || 0,
  };
}

function makeTPRDailyBreakdownResponse(
  json: any
): GetTPRDailyBreakdownResponse {
  return {
    days: (json?.days || []).map(makeTPRDailyPoint),
    rangeStart: Number(json?.rangeStart) || 0,
    rangeEnd: Number(json?.rangeEnd) || 0,
    calibratingAccounts: json.calibratingAccounts || 0,
  };
}

function makeTPRDailyPoint(json: any): TPRDailyPoint {
  return {
    day: Number(json?.day) || 0,
    periodAvg: Number(json?.periodAvg) || 0,
    contributingClusters: json?.contributingClusters || 0,
    metrics: Object.fromEntries(
      Object.entries(json?.metrics || {}).map(([k, v]) => [k, Number(v)])
    ),
  };
}

function makeStripeConfigResponse(json: any): GetStripeConfigResponse {
  return {
    publicKey: json?.publicKey || '',
    stripeCustomerId: json?.stripeCustomerId || '',
  };
}

function makeStripeCard(json: any): StripeCard {
  return {
    id: json?.id || '',
    last4: json?.last4 || '',
    addressLine1: json?.addressLine1 || '',
    addressLine2: json?.addressLine2 || '',
    city: json?.city || '',
    country: json?.country || '',
    state: json?.state || '',
    name: json?.name || '',
    zip: json?.zip || '',
    brand: json?.brand || '',
    expirationMonth: Number(json?.expirationMonth) || 0,
    expirationYear: Number(json?.expirationYear) || 0,
    createdAt: Number(json?.createdAt) || 0,
  };
}

function makeStripeListCardsResponse(json: any): StripeListCardsResponse {
  return {
    stripeCards: (json?.stripeCards || []).map(makeStripeCard),
    stripeDefaultSourceId: json?.stripeDefaultSourceId || '',
    stripeMissingPaymentMethod: !!json?.stripeMissingPaymentMethod,
  };
}

function makeStripeCreateSetupIntentResponse(
  json: any
): StripeCreateSetupIntentResponse {
  return {
    clientSecret: json?.clientSecret || '',
  };
}

function makeStripeInvoice(json: any): StripeInvoice {
  return {
    invoiceId: json?.invoiceId || '',
    status: json?.status || '',
    amountDue: Number(json?.amountDue) || 0,
    amountPaid: Number(json?.amountPaid) || 0,
    periodEnd: Number(json?.periodEnd) || 0,
    periodStart: Number(json?.periodStart) || 0,
    invoicePdf: json?.invoicePdf || '',
  };
}

function makeStripeListInvoicesResponse(json: any): StripeListInvoicesResponse {
  return {
    stripeInvoices: (json?.stripeInvoices || []).map(makeStripeInvoice),
  };
}

function makeStripeGetSettingsResponse(json: any): StripeGetSettingsResponse {
  return {
    stripeInvoiceBillingAddress: json?.stripeInvoiceBillingAddress
      ? {
          addressCity: json.stripeInvoiceBillingAddress.addressCity || '',
          addressCountry: json.stripeInvoiceBillingAddress.addressCountry || '',
          addressLine1: json.stripeInvoiceBillingAddress.addressLine1 || '',
          addressLine2: json.stripeInvoiceBillingAddress.addressLine2 || '',
          addressPostalCode:
            json.stripeInvoiceBillingAddress.addressPostalCode || '',
          addressState: json.stripeInvoiceBillingAddress.addressState || '',
        }
      : undefined,
    stripeInvoiceEmail: json?.stripeInvoiceEmail || '',
    stripeInvoicePrefix: json?.stripeInvoicePrefix || '',
    stripeCustomerName: json?.stripeCustomerName || '',
    stripeSubscriptionStatus: json?.stripeSubscriptionStatus || '',
    planName: json?.planName || '',
    stripeTrialEnd: json?.stripeTrialEnd || 0,
  };
}

function buildBreakdownQueryParams(
  req: GetMAUDailyBreakdownRequest | GetTPRDailyBreakdownRequest
): string {
  const params = new URLSearchParams();

  for (const tenant of req.tenants) {
    params.append('tenants', tenant);
  }

  const w = req.window?.window;
  if (w && 'cycle' in w) {
    params.set('window-cycle', String(w.cycle));
  } else if (w && 'range' in w) {
    params.set('window-range-start', String(w.range.start));
    params.set('window-range-end', String(w.range.end));
  }

  return params.toString();
}
