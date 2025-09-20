/**
 *
 * @param cloud boolean value indicating customer is on a Teleport Cloud subscription
 * @param cycleStart unix timestamp of the start of the cycle
 * @param cycleEnd unix timestamp of the end of the cycle
 * @param hasCloudAnonymizationKey whether the account has a Cloud Anonymization key or not
 * @param salesforceIdUpdatedAt unix timestamp of when the SFID of the account was updated
 * @returns true if the cycle is a calibration period
 */
export const isCalibrationPeriod = (
  cloud: boolean,
  cycleStart: number,
  cycleEnd: number,
  hasCloudAnonymizationKey: boolean,
  salesforceIdUpdatedAt: number
): boolean => {
  return (
    cloud &&
    hasCloudAnonymizationKey &&
    salesforceIdUpdatedAt <= cycleEnd &&
    salesforceIdUpdatedAt >= cycleStart
  );
};
