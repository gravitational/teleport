import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

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
      stripePublicKey: 'some-stripePublicKey',
      stripeCustomerId: 'some-stripeCustomerId',
      stripeCurrentUsage: {
        invoiceId: 'some-invoiceId',
        status: 'some-status',
        periodEnd: 1684773766,
        periodStart: 1684773766,
        usageMau: 0,
        usagePr: 2,
      },
      stripeTrial: true,
      stripeTrialEnd: 1684773766,
      stripeMissingPaymentMethod: false,
      productName: 'some-productName',
      stripeSubscriptionStatus: 'some-stripeSubscriptionStatus',
      stripeSubscriptionCancelAt: 1684773766,
      stripeSubscriptionCanceledAt: 1684773766,
      usageUpdatedAt: 0,
    };
    jest.spyOn(api, 'get').mockResolvedValue(expected);

    let response = await cloud.fetchBillingSummaryInformation();
    expect(api.get).toHaveBeenCalledWith(cfg.api.billingSummaryPath);
    expect(response).toEqual(expected);
  });
});
