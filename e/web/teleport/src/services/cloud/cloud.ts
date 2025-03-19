import cfg from 'e-teleport/config';
import api from 'teleport/services/api';
import auth from 'teleport/services/auth/auth';
import { User } from 'teleport/services/user/types';

import {
  BillingSummaryInformation,
  NonBillableSummaryInformation,
  SendTeleportCredentialReset,
  SendTeleportInvite,
} from './types';

class CloudService {
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

function makeBillingSummaryInformation(json: any): BillingSummaryInformation {
  const { usageHistory } = json.usageSummary;
  return {
    ...json,
    usageSummary: {
      ...json.usageSummary,
      usageHistory: usageHistory.map(u => ({
        ...u,
        mau: parseInt(u.mau) || 0,
        tpr: parseInt(u.tpr) || 0,
        mwi: parseInt(u.mwi) || 0,
        igmau: parseInt(u.ig_mau) || 0,
      })),
    },
  };
}

function makeNonBillableUsageSummary(json: any) {
  return json as NonBillableSummaryInformation;
}
