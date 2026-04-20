import cfg from 'e-teleport/config';
import {
  GetMAUDailyBreakdownRequest,
  GetMAUDailyBreakdownResponse,
  GetTPRDailyBreakdownRequest,
  GetTPRDailyBreakdownResponse,
  GetUsageRequest,
  GetUsageResponse,
  MAUDailyPoint,
  TPRDailyPoint,
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
