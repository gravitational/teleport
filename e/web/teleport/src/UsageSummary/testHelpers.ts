import { getUnixTime } from 'date-fns';

import {
  GetBillingSummaryInformationResponse,
  UsageHistoryItem,
  UsageSummary,
} from 'e-teleport/services/cloud/v1/tenants_pb';

export const makeGetBillingSummaryInformationResponse = (
  overrides: Partial<GetBillingSummaryInformationResponse> = {}
): GetBillingSummaryInformationResponse => {
  return Object.assign(
    {
      usageBasedBilling: true,
      productName: 'Enterprise',
      usageQuota: {
        mauMax: 300,
        tprMax: 222,
        mauInc: 67,
        tprInc: 9000,
      },
      salesforceIdUpdatedAt: new Date('2024/09/01').getTime(),
      usageUpdatedAt: new Date('2024/01/02').getTime(),
      usageSummary: makeUsageSummary(),
    },
    overrides
  );
};

export const makeUsageSummary = (
  overrides: Partial<UsageSummary> = {}
): UsageSummary => {
  return Object.assign(
    {
      cloud: false,
      cycleEnd: new Date('2024/01/30').getTime() / 1000,
      cycleEndFormatted: 'Jan 30, 2024',
      cycleStart: new Date('2024/01/02').getTime() / 1000,
      cycleStartFormatted: 'Jan 02, 2024',
      hasCloudAnonymizationKey: false,
      salesforceIdUpdatedAt: new Date('2024/09/01').getTime() / 1000,
      salesforceIdUpdatedAtFormatted: 'Jan 09, 2024',
      usageBased: false,
      usageUpdatedAt: new Date('2024/01/02').getTime() / 1000,
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
      mwi: {
        cycleCount: 0,
        maximum: 0,
        free: 0,
        perMau: 0,
      },
      igmau: {
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

export const usageHistory: UsageHistoryItem[] = [
  {
    mau: 102,
    tpr: 1002,
    mwi: 52,
    igmau: 42,
    cycleStart: getUnixTime(new Date('2023/03/15')),
    cycleStartFormatted: 'Mar 15, 2023',
    cycleEnd: getUnixTime(new Date('2023/04/14')),
    cycleEndFormatted: 'Apr 14, 2023',
  },
  {
    mau: 101,
    tpr: 1001,
    mwi: 51,
    igmau: 41,
    cycleStart: getUnixTime(new Date('2023/02/15')),
    cycleStartFormatted: 'Feb 15, 2023',
    cycleEnd: getUnixTime(new Date('2023/03/14')),
    cycleEndFormatted: 'Mar 14, 2023',
  },
  {
    mau: 100,
    tpr: 1000,
    mwi: 20,
    igmau: 10,
    cycleStart: getUnixTime(new Date('2023/01/15')),
    cycleStartFormatted: 'Jan 15, 2023',
    cycleEnd: getUnixTime(new Date('2023/02/14')),
    cycleEndFormatted: 'Feb 14, 2023',
  },
];
