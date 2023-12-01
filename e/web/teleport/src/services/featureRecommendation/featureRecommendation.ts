import { RecommendationStatus } from 'teleport/types';

import { storageService } from 'teleport/services/storageService';
import {
  Feature,
  FeatureRecommendationStatus,
  userEventService,
} from 'teleport/services/userEvent';

import CloudService from 'e-teleport/services/cloud';

/**
 * setAndEmitFeatureRecommendationStatus sets feature recommendation state in local storage and
 * emits featureRecommendationEvent. setAndEmitFeatureRecommendationStatus should only be called
 * in Team Plan (cfg.isTeam) and for users with billing role (getFeatureFlags().billing).
 * @param {CloudService} cloudService - is the cloud service used to fetch fetchNonBillableSummaryInformation.
 */
export async function setAndEmitFeatureRecommendationStatus(
  cloudService: CloudService
) {
  const recommendFeatureCurrentState =
    storageService.getFeatureRecommendationStatus();

  // Initializes empty local storage state for feature recommendation statuses.
  // Emits "notified" event only if `devicesInUse` is == 0.
  if (!recommendFeatureCurrentState?.TrustedDevices) {
    // explicitly check devicesInUse. Otherwise, even though the local storage object is not
    // set yet, we may end up showing "red dot" and emitting event for Team Plan users
    // who may be using Device Trust before this feature is shipped or another device admin
    // may have already enrolled their device.
    const { trustedDeviceUsage } =
      await cloudService.fetchNonBillableSummaryInformation();
    const trustedDevicesRecommendationStatus = trustedDeviceUsage.devicesInUse
      ? RecommendationStatus.Done
      : RecommendationStatus.Notify;

    // preserve the current status of trustedDevicesRecommendationStatus
    storageService.setRecommendFeature({
      TrustedDevices: trustedDevicesRecommendationStatus,
    });

    if (trustedDevicesRecommendationStatus == RecommendationStatus.Notify) {
      userEventService.captureFeatureRecommendationEvent({
        Feature: Feature.FEATURES_TRUSTED_DEVICES,
        FeatureRecommendationStatus:
          FeatureRecommendationStatus.FEATURE_RECOMMENDATION_STATUS_NOTIFIED,
      });
    }
  } else if (
    recommendFeatureCurrentState?.TrustedDevices !== RecommendationStatus.Done
  ) {
    // if local storage object exists and the state is not DONE, emits DONE event and
    // sets local storage state to DONE only if `devicesInUse` > 0
    const { trustedDeviceUsage } =
      await cloudService.fetchNonBillableSummaryInformation();

    // TODO(sshah): update to status DONE once user completes desired CTA from within the UI.
    const trustedDevicesRecommendationStatus = trustedDeviceUsage.devicesInUse
      ? RecommendationStatus.Done
      : RecommendationStatus.Notify;

    // preserve the current status of trustedDevicesRecommendationStatus
    storageService.setRecommendFeature({
      TrustedDevices: trustedDevicesRecommendationStatus,
    });

    if (trustedDevicesRecommendationStatus === RecommendationStatus.Done) {
      userEventService.captureFeatureRecommendationEvent({
        Feature: Feature.FEATURES_TRUSTED_DEVICES,
        FeatureRecommendationStatus:
          FeatureRecommendationStatus.FEATURE_RECOMMENDATION_STATUS_DONE,
      });
    }
  }
}
