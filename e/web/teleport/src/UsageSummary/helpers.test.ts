import { getUnixTime } from 'date-fns';

import { isCalibrationPeriod } from 'e-teleport/UsageSummary/helpers';

describe('isCalibrationPeriod', () => {
  test('returns true when all conditions are met', () => {
    const cycleStart = getUnixTime(new Date('2022/01/15'));
    const cycleEnd = getUnixTime(new Date('2022/02/15'));
    const hasCloudAnonymizationKey = true;
    const salesforceIdUpdatedAt = getUnixTime(new Date('2022/01/31'));

    const result = isCalibrationPeriod(
      true,
      cycleStart,
      cycleEnd,
      hasCloudAnonymizationKey,
      salesforceIdUpdatedAt
    );

    expect(result).toBe(true);
  });

  test('returns false if hasCloudAnonymizationKey is false', () => {
    const cycleStart = getUnixTime(new Date('2022/01/15'));
    const cycleEnd = getUnixTime(new Date('2022/02/15'));
    const hasCloudAnonymizationKey = false;
    const salesforceIdUpdatedAt = getUnixTime(new Date('2022/01/31'));

    const result = isCalibrationPeriod(
      true,
      cycleStart,
      cycleEnd,
      hasCloudAnonymizationKey,
      salesforceIdUpdatedAt
    );

    expect(result).toBe(false);
  });

  test('returns false if salesforceIdUpdatedAt is not within the cycle period', () => {
    const cycleStart = getUnixTime(new Date('2022/01/15'));
    const cycleEnd = getUnixTime(new Date('2022/02/15'));
    const hasCloudAnonymizationKey = true;
    const salesforceIdUpdatedAt = getUnixTime(new Date('2022/01/01'));

    const result = isCalibrationPeriod(
      true,
      cycleStart,
      cycleEnd,
      hasCloudAnonymizationKey,
      salesforceIdUpdatedAt
    );

    expect(result).toBe(false);
  });

  test('returns false if not cloud', () => {
    const cycleStart = getUnixTime(new Date('2022/01/15'));
    const cycleEnd = getUnixTime(new Date('2022/02/15'));
    const hasCloudAnonymizationKey = true;
    const salesforceIdUpdatedAt = getUnixTime(new Date('2022/01/31'));

    const result = isCalibrationPeriod(
      false,
      cycleStart,
      cycleEnd,
      hasCloudAnonymizationKey,
      salesforceIdUpdatedAt
    );

    expect(result).toBe(false);
  });
});
