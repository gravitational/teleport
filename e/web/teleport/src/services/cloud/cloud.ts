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
    const webauthnResponse = await auth.getWebauthnResponseForAdminAction(true);
    return api.post(cfg.api.teleportInvitePath, req, null, webauthnResponse);
  }

  async sendTeleportCredentialReset(
    req: SendTeleportCredentialReset
  ): Promise<void> {
    const webauthnResponse = await auth.getWebauthnResponseForAdminAction(true);
    return api.post(
      cfg.api.teleportCredentialResetPath,
      req,
      null,
      webauthnResponse
    );
  }
}

export default CloudService;

function makeBillingSummaryInformation(json: any) {
  return json as BillingSummaryInformation;
}

function makeNonBillableUsageSummary(json: any) {
  return json as NonBillableSummaryInformation;
}
