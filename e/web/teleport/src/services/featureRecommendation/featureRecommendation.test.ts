import {
  Feature,
  FeatureRecommendationStatus,
  userEventService,
} from 'teleport/services/userEvent';

import { RecommendationStatus } from 'teleport/types';
import { KeysEnum } from 'teleport/services/storageService/types';

import CloudService from 'e-teleport/services/cloud';

import { setAndEmitFeatureRecommendationStatus } from './featureRecommendation';

describe('setAndEmitFeatureRecommendationStatus', () => {
  const cloudService = new CloudService();

  beforeEach(() => {
    jest.spyOn(userEventService, 'captureFeatureRecommendationEvent');
  });

  afterEach(() => jest.resetAllMocks());

  test('no existing local state but devicesInUse is 1: DONE is set but event is not fired', async () => {
    // default state: no local storage set for KeysEnum.RECOMMEND_FEATURE
    let recommendFeatureCurrentState = {
      TrustedDevices: null,
    };
    localStorage.setItem(
      KeysEnum.RECOMMEND_FEATURE,
      JSON.stringify(recommendFeatureCurrentState)
    );

    // resolve 1 devicesInUse
    jest
      .spyOn(cloudService, 'fetchNonBillableSummaryInformation')
      .mockResolvedValue({
        trustedDeviceUsage: {
          devicesUsageLimit: 5,
          devicesInUse: 1,
        },
        accessRequestUsage: { monthlyLimit: 0, monthlyUsed: 0 },
      });

    await setAndEmitFeatureRecommendationStatus(cloudService);

    // localstorage should be updated with NOTIFY state
    const localState = localStorage.getItem(KeysEnum.RECOMMEND_FEATURE);
    recommendFeatureCurrentState.TrustedDevices = RecommendationStatus.Done;
    expect(JSON.parse(localState)).toStrictEqual(recommendFeatureCurrentState);

    // captureFeatureRecommendationEvent should be emitted with NOTIFIED state
    expect(
      userEventService.captureFeatureRecommendationEvent
    ).not.toHaveBeenCalledWith();
  });

  test('no existing local state and devicesInUse is 0: NOTIFY is set and NOTIFIED is fired', async () => {
    // default state: no local storage set for KeysEnum.RECOMMEND_FEATURE
    let recommendFeatureCurrentState = {
      TrustedDevices: null,
    };
    localStorage.setItem(
      KeysEnum.RECOMMEND_FEATURE,
      JSON.stringify(recommendFeatureCurrentState)
    );

    // resolve 0 devicesInUse
    jest
      .spyOn(cloudService, 'fetchNonBillableSummaryInformation')
      .mockResolvedValue({
        trustedDeviceUsage: {
          devicesUsageLimit: 5,
          devicesInUse: 0,
        },
        accessRequestUsage: { monthlyLimit: 0, monthlyUsed: 0 },
      });

    await setAndEmitFeatureRecommendationStatus(cloudService);

    // localstorage should be updated with NOTIFY state
    const localState = localStorage.getItem(KeysEnum.RECOMMEND_FEATURE);
    recommendFeatureCurrentState.TrustedDevices = RecommendationStatus.Notify;
    expect(JSON.parse(localState)).toStrictEqual(recommendFeatureCurrentState);

    // captureFeatureRecommendationEvent should be emitted with NOTIFIED state
    expect(
      userEventService.captureFeatureRecommendationEvent
    ).toHaveBeenCalledWith({
      Feature: Feature.FEATURES_TRUSTED_DEVICES,
      FeatureRecommendationStatus:
        FeatureRecommendationStatus.FEATURE_RECOMMENDATION_STATUS_NOTIFIED,
    });
  });

  test('local state is NOTIFY and devicesInUse is 1: DONE is set and fired', async () => {
    // local state with NOTIFY
    let recommendFeatureCurrentState = {
      TrustedDevices: RecommendationStatus.Notify,
    };
    localStorage.setItem(
      KeysEnum.RECOMMEND_FEATURE,
      JSON.stringify(recommendFeatureCurrentState)
    );

    // resolve 1 devicesInUse
    jest
      .spyOn(cloudService, 'fetchNonBillableSummaryInformation')
      .mockResolvedValue({
        trustedDeviceUsage: {
          devicesUsageLimit: 5,
          devicesInUse: 1,
        },
        accessRequestUsage: { monthlyLimit: 0, monthlyUsed: 0 },
      });

    await setAndEmitFeatureRecommendationStatus(cloudService);

    // localstorage should be updated with DONE state
    const localStatus = localStorage.getItem(KeysEnum.RECOMMEND_FEATURE);
    recommendFeatureCurrentState.TrustedDevices = RecommendationStatus.Done;
    expect(JSON.parse(localStatus)).toStrictEqual(recommendFeatureCurrentState);

    // captureFeatureRecommendationEvent should be emitted with DONE status
    expect(
      userEventService.captureFeatureRecommendationEvent
    ).toHaveBeenCalledWith({
      Feature: Feature.FEATURES_TRUSTED_DEVICES,
      FeatureRecommendationStatus:
        FeatureRecommendationStatus.FEATURE_RECOMMENDATION_STATUS_DONE,
    });
  });

  test('local state is NOTIFY with zero devicesInUse: status is not updated and event is not fired', async () => {
    // local state with NOTIFY
    const recommendFeatureCurrentState = {
      TrustedDevices: RecommendationStatus.Notify,
    };
    localStorage.setItem(
      KeysEnum.RECOMMEND_FEATURE,
      JSON.stringify(recommendFeatureCurrentState)
    );

    // resolve 0 devicesInUse
    jest
      .spyOn(cloudService, 'fetchNonBillableSummaryInformation')
      .mockResolvedValue({
        trustedDeviceUsage: {
          devicesUsageLimit: 5,
          devicesInUse: 0,
        },
        accessRequestUsage: { monthlyLimit: 0, monthlyUsed: 0 },
      });

    await setAndEmitFeatureRecommendationStatus(cloudService);
    // localstorage should be be the same
    const localStatus = localStorage.getItem(KeysEnum.RECOMMEND_FEATURE);
    expect(JSON.parse(localStatus)).toStrictEqual(recommendFeatureCurrentState);
    // captureFeatureRecommendationEvent should not be emitted as there isn't any change in devicesInUse
    expect(
      userEventService.captureFeatureRecommendationEvent
    ).not.toHaveBeenCalledWith();
  });

  test('local state is DONE: no event is set and fired.', async () => {
    // local state with DONE
    const recommendFeatureCurrentState = {
      TrustedDevices: RecommendationStatus.Done,
    };
    localStorage.setItem(
      KeysEnum.RECOMMEND_FEATURE,
      JSON.stringify(recommendFeatureCurrentState)
    );

    // fetchNonBillableSummaryInformation should not be called as the recommendFeatureCurrentState is DONE
    jest.spyOn(cloudService, 'fetchNonBillableSummaryInformation');
    expect(
      cloudService.fetchNonBillableSummaryInformation
    ).not.toHaveBeenCalled();

    await setAndEmitFeatureRecommendationStatus(cloudService);

    // localstorage should be be the same
    const localStatus = localStorage.getItem(KeysEnum.RECOMMEND_FEATURE);
    expect(JSON.parse(localStatus)).toStrictEqual(recommendFeatureCurrentState);

    // captureFeatureRecommendationEvent should not be emitted as the recommendFeatureCurrentState is DONE
    expect(
      userEventService.captureFeatureRecommendationEvent
    ).not.toHaveBeenCalled();
  });
});
