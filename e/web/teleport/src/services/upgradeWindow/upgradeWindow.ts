import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

export const availableUpgradeWindowStartHours = [8, 16, 23] as const;
export type UpgradeWindowStartHour =
  typeof availableUpgradeWindowStartHours[number];

export const service = {
  getUpgradeWindowStartHour(): Promise<UpgradeWindowStartHour> {
    return api
      .get(cfg.api.upgradeWindowStartPath)
      .then(res => res.upgradeWindowStart);
  },

  updateUpgradeWindowStart(
    upgradeWindowStart: UpgradeWindowStartHour
  ): Promise<UpgradeWindowStartHour> {
    return api
      .post(cfg.api.upgradeWindowStartPath, {
        upgradeWindowStart,
      })
      .then(res => res.windowStart);
  },
};
