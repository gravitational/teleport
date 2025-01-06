import cfg from 'e-teleport/config';
import api from 'teleport/services/api';

import CloudSvc from './cloud';
import { BillingSummaryInformation } from './types';

describe('cloudService', () => {
  let cloud: CloudSvc;

  beforeEach(() => {
    jest.clearAllMocks();
    cloud = new CloudSvc();
  });

  test('fetchBillingSummaryInformation', async () => {
    const expected: BillingSummaryInformation = {
      usageBasedBilling: false,
      stripeCurrentUsage: {
        invoiceId: 'some-invoiceId',
        status: 'some-status',
        periodEnd: 1684773766,
        periodStart: 1684773766,
        usageMau: 0,
        usagePr: 2,
      },
      productName: 'some-productName',
      usageUpdatedAt: 0,
      salesforceIdUpdatedAt: new Date('2024/09/01').getTime(),
      usageSummary: {
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
          maximum: 1,
          free: 2,
          cycleCount: 3,
          perMau: 0,
        },
        tpr: {
          maximum: 4,
          free: 5,
          cycleCount: 6,
          perMau: 7,
        },
        usageHistory: [],
      },
    };
    jest.spyOn(api, 'get').mockResolvedValue(expected);

    let response = await cloud.fetchBillingSummaryInformation();
    expect(api.get).toHaveBeenCalledWith(cfg.api.billingSummaryPath);
    expect(response).toEqual(expected);
  });
});
