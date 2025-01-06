import { UsageSummary } from 'e-teleport/services/cloud/v1/tenants_pb';

export const makeUsageSummary = (
  overrides: Partial<UsageSummary> = {}
): UsageSummary => {
  return Object.assign(
    {
      cloud: false,
      cycleEnd: new Date('2024/01/30').getTime(),
      cycleEndFormatted: 'Jan 30, 2024',
      cycleStart: new Date('2024/01/02').getTime(),
      cycleStartFormatted: 'Jan 02, 2024',
      hasCloudAnonymizationKey: false,
      salesforceIdUpdatedAt: new Date('2024/09/01').getTime(),
      salesforceIdUpdatedAtFormatted: 'Jan 09, 2024',
      usageBased: false,
      usageUpdatedAt: new Date('2024/01/02').getTime(),
      usageUpdatedAtFormatted: 'Jan 02, 2024',
      mau: {
        cycleCount: 0,
        maximum: 0,
        free: 0,
        perMau: 0,
      },
      tpr: {
        cycleCount: 0,
        maximum: 0,
        free: 0,
        perMau: 0,
      },
      usageHistory: [],
    },
    overrides
  );
};
