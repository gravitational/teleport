import { getUnixTime } from 'date-fns';

import { render, screen } from 'design/utils/testing';

import {
  isCalibrationPeriod,
  SummaryPage,
} from 'e-teleport/UsageSummary/SummaryPage';
import { makeUsageSummary } from 'e-teleport/UsageSummary/testHelpers';

jest.mock('teleport/useStickyClusterId', () =>
  jest.fn(() => ({ clusterId: 'cluster-name', isLeafCluster: false }))
);

test('renders cycle if cycle usage is present', () => {
  render(
    <SummaryPage
      summary={makeUsageSummary({
        usageBased: true,
        mau: {
          maximum: 2,
          free: 2,
          cycleCount: 2,
          perMau: 0,
        },
        tpr: {
          maximum: 2,
          free: 2,
          cycleCount: 2,
          perMau: 0,
        },
      })}
    />
  );

  expect(screen.getByText(/Current Cycle:/i)).toBeInTheDocument();
});

test('does not render cycle if cycle usage is not present', () => {
  render(<SummaryPage summary={undefined} />);

  expect(screen.queryByText(/Current Cycle/i)).not.toBeInTheDocument();
  expect(
    screen.getByText(/Usage data is being gathered./i)
  ).toBeInTheDocument();
});

describe('isCalibrationPeriod', () => {
  test('returns true when all conditions are met', () => {
    const cycleStart = getUnixTime(new Date('2022/01/15'));
    const cycleEnd = getUnixTime(new Date('2022/02/15'));
    const hasCloudAnonymizationKey = true;
    const salesforceIdUpdatedAt = getUnixTime(new Date('2022/01/31'));

    const result = isCalibrationPeriod(
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
      cycleStart,
      cycleEnd,
      hasCloudAnonymizationKey,
      salesforceIdUpdatedAt
    );

    expect(result).toBe(false);
  });
});
