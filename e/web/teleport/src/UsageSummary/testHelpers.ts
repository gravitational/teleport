import {
  GetUsageResponse,
  PricingModel,
  Usage,
  UsageCycle,
  UsageLimits,
} from 'e-teleport/services/cloud/v1/tenants_pb';
import { Metric } from 'e-teleport/UsageSummary/Summary';

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
      pricingModel: makePricingModel(),
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

export const makePricingModel = (
  overrides: Partial<PricingModel> = {}
): PricingModel => {
  return Object.assign(
    {
      version: '',
      modelId: '',
      name: '',
      createdAt: 0,
      description: '',
      metric: [Metric.MWI, Metric.IGMAU, Metric.ISTPR, Metric.MAU, Metric.TPR],
    },
    overrides
  );
};

function toUnixDateTestHelper(date: Date): number {
  return Math.floor(date.getTime() / 1000);
}
