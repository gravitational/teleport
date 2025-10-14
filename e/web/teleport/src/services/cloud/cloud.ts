import cfg from 'e-teleport/config';
import {
  GetUsageRequest,
  GetUsageResponse,
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
