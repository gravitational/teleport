import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

export const availableUpgradeWindowStarts = [
  '08:00:00',
  '16:00:00',
  '23:00:00',
] as const;
export type UpgradeWindowStart = typeof availableUpgradeWindowStarts[number];

export const service = {
  getUpgradeWindowStart(): Promise<UpgradeWindowStart> {
    return api
      .get(cfg.api.upgradeWindowStartPath)
      .then(res => res.upgradeWindowStart);
  },

  updateUpgradeWindowStart(
    upgradeWindowStart: UpgradeWindowStart
  ): Promise<UpgradeWindowStart> {
    return api
      .post(cfg.api.upgradeWindowStartPath, {
        upgradeWindowStart,
      })
      .then(res => res.windowStart);
  },
};
