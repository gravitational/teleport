import cfg from 'e-teleport/config';
import { GetUsageResponse } from 'e-teleport/services/cloud/v1/tenants_pb';
import api from 'teleport/services/api';

import CloudSvc from './cloud';

describe('cloudService', () => {
  let cloud: CloudSvc;

  beforeEach(() => {
    jest.clearAllMocks();
    cloud = new CloudSvc();
  });

  test('fetchBillingSummaryInformation', async () => {
    const expected: GetUsageResponse = {
      aggregateCount: 1,
      usageUpdatedAt: 0,
      alerts: [],
      missingEntitlements: [],
      usageHistory: [
        {
          activeAccounts: 1,
          calibratingAccounts: 0,
          end: new Date('2024/01/30').getTime(),
          endFormatted: 'Jan 30, 2024',
          pricingModel: {
            version: '4.0.0',
            modelId: '1a5451df-74e1-4332-b3b9-e397bdd29e02',
            name: 'model four',
            createdAt: 1762379155000,
            description: 'the newest pricing model',
            metric: ['MWI', 'IGMAU', 'ISTPR', 'MAU', 'TPR'],
          },
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
    };
    jest.spyOn(api, 'post').mockResolvedValue(expected);

    let response = await cloud.fetchBillingSummaryInformation({ tenants: [] });
    expect(api.post).toHaveBeenCalledWith(cfg.api.billingSummaryPath, {
      tenants: [],
    });
    expect(response).toEqual(expected);
  });

  test('getUpgradeWindowStartHour', async () => {
    jest.spyOn(api, 'get').mockResolvedValue({ upgradeWindowStartHour: 8 });

    let response = await cloud.getUpgradeWindowStartHour('clusterId');
    expect(api.get).toHaveBeenCalledWith(
      cfg.getWindowUpgradeStartUrl('clusterId')
    );
    expect(response).toEqual(8);
  });

  test('updateUpgradeWindowStart', async () => {
    jest.spyOn(api, 'post').mockResolvedValue({ upgradeWindowStartHour: 8 });

    let response = await cloud.updateUpgradeWindowStart('clusterId', 8);
    expect(api.post).toHaveBeenCalledWith(
      cfg.getWindowUpgradeStartUrl('clusterId'),
      { upgradeWindowStartHour: 8 }
    );
    expect(response).toEqual(8);
  });

  test('getEnvironmentProfile', async () => {
    jest
      .spyOn(api, 'get')
      .mockResolvedValue({ environmentProfile: 'production' });

    let response = await cloud.getEnvironmentProfile();
    expect(api.get).toHaveBeenCalledWith(cfg.api.environmentProfileUrl);
    expect(response).toEqual({ environmentProfile: 'production' });
  });

  test('updateEnvironmentProfile', async () => {
    jest
      .spyOn(api, 'post')
      .mockResolvedValue({ environmentProfile: 'production' });

    let response = await cloud.updateEnvironmentProfile('production');
    expect(api.post).toHaveBeenCalledWith(cfg.api.environmentProfileUrl, {
      environmentProfile: 'production',
    });
    expect(response).toEqual({ environmentProfile: 'production' });
  });
});
