import { getUnixTime } from 'date-fns';
import type { ReactNode } from 'react';

import { render, screen } from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  isCalibrationPeriod,
  SummaryPage,
} from 'e-teleport/UsageSummary/SummaryPage';
import { makeUsageSummary } from 'e-teleport/UsageSummary/testHelpers';
import { ContextProvider } from 'teleport/index';

function renderWithContext(component: ReactNode) {
  const ctx = createTeleportContextE();

  return render(<ContextProvider ctx={ctx}>{component}</ContextProvider>);
}

jest.mock('teleport/useStickyClusterId', () =>
  jest.fn(() => ({ clusterId: 'cluster-name', isLeafCluster: false }))
);

test('renders cycle if cycle usage is present', () => {
  renderWithContext(
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
        mwi: {
          maximum: 2,
          free: 2,
          cycleCount: 2,
          perMau: 0,
        },
        igmau: {
          maximum: 2,
          free: 2,
          cycleCount: 2,
          perMau: 0,
        },
      })}
      hasIdentityGovernance={true}
      hasIdentitySecurity={true}
    />
  );

  expect(screen.getByText(/Current Billing Cycle:/i)).toBeInTheDocument();
});

test('does not render cycle if cycle usage is not present', () => {
  renderWithContext(
    <SummaryPage
      summary={undefined}
      hasIdentityGovernance={true}
      hasIdentitySecurity={true}
    />
  );

  expect(screen.queryByText(/Current Billing Cycle/i)).not.toBeInTheDocument();
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
