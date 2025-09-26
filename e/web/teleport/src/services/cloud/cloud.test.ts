import cfg from 'e-teleport/config';
import { GetUsageResponse } from 'e-teleport/services/cloud/v1/tenants_pb';
import { makeGetUsageResponse } from 'e-teleport/UsageSummary/testHelpers';
import api from 'teleport/services/api';

import CloudSvc from './cloud';

describe('cloudService', () => {
  let cloud: CloudSvc;

  beforeEach(() => {
    jest.clearAllMocks();
    cloud = new CloudSvc();
  });

  test('fetchBillingSummaryInformation', async () => {
    const expected: GetUsageResponse = makeGetUsageResponse({
      aggregateCount: 1,
      usageUpdatedAt: 0,
      usageHistory: [
        {
          activeAccounts: 1,
          calibratingAccounts: 0,
          end: new Date('2024/01/30').getTime(),
          endFormatted: 'Jan 30, 2024',
          start: new Date('2024/01/02').getTime(),
          startFormatted: 'Jan 02, 2024',
          usage: {
            ztamau: 1,
            igmau: 3,
            mwi: 33,
            tpr: 12,
          },
          usageLimits: {
            ztamau: 2,
            igmau: 4,
            mwi: 35,
            tpr: 10,
          },
        },
      ],
    });
    jest.spyOn(api, 'post').mockResolvedValue(expected);

    let response = await cloud.fetchBillingSummaryInformation({ tenants: [] });
    expect(api.post).toHaveBeenCalledWith(cfg.api.billingSummaryPath, {
      tenants: [],
    });
    expect(response).toEqual(expected);
  });
});
