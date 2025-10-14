import {
  GetUsageResponse,
  Usage,
  UsageCycle,
  UsageLimits,
} from 'e-teleport/services/cloud/v1/tenants_pb';

export const makeGetUsageResponse = (
  overrides: Partial<GetUsageResponse> = {}
): GetUsageResponse => {
  return Object.assign(
    {
      alerts: [],
      usageHistory: [],
      missingEntitlements: [],
      aggregateCount: 0,
      usageUpdatedAt: 0,
    },
    overrides
  );
};

export const makeUsageCycle = (
  overrides: Partial<UsageCycle> = {}
): UsageCycle => {
  return Object.assign(
    {
      usage: makeUsage(),
      usageLimits: makeUsageLimits(),
      start: toUnixDateTestHelper(new Date('2024/01/02')),
      startFormatted: 'Jan 02, 2024',
      end: toUnixDateTestHelper(new Date('2024/02/01')),
      endFormatted: 'Feb 01, 2024',
      activeAccounts: 0,
      calibratingAccounts: 0,
    },
    overrides
  );
};

export const makeUsage = (overrides: Partial<Usage> = {}): Usage => {
  return Object.assign(
    {
      igmau: 0,
      mwi: 0,
      tpr: 0,
      ztamau: 0,
    },
    overrides
  );
};

export const makeUsageLimits = (
  overrides: Partial<UsageLimits> = {}
): UsageLimits => {
  return Object.assign(
    {
      igmau: 0,
      mwi: 0,
      tpr: 0,
      ztamau: 0,
    },
    overrides
  );
};

function toUnixDateTestHelper(date: Date): number {
  return Math.floor(date.getTime() / 1000);
}
