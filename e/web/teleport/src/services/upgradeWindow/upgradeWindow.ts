import api from 'teleport/services/api';

import cfg from 'e-teleport/config';
import {
  GetAccountUpgradeWindowStartHourResponse,
  UpdateAccountUpgradeWindowStartHourRequest,
} from 'e-teleport/services/cloud';

export const availableUpgradeWindowStartHours = [8, 16, 23] as const;
export type UpgradeWindowStartHour =
  (typeof availableUpgradeWindowStartHours)[number];

export const service = {
  getUpgradeWindowStartHour(clusterId): Promise<UpgradeWindowStartHour> {
    return api
      .get(cfg.getWindowUpgradeStartUrl(clusterId))
      .then(
        (res: GetAccountUpgradeWindowStartHourResponse) =>
          res.upgradeWindowStartHour as UpgradeWindowStartHour
      );
  },

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
  },
};
