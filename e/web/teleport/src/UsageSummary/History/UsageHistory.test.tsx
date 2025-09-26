import { render, screen } from 'design/utils/testing';

import {
  makeGetUsageResponse,
  makeUsage,
  makeUsageCycle,
} from '../testHelpers';
import { UsageHistory } from './UsageHistory';

const calibrationInfo =
  /A change to your account required a calibration period in order to accurately count Active Users across trusted clusters/;

test('shows empty state when there is no usage history', async () => {
  render(<UsageHistory usageResponse={makeGetUsageResponse()} />);

  expect(
    await screen.findByText('No cycle information available')
  ).toBeInTheDocument();
});

test('renders all elements', async () => {
  render(
    <UsageHistory
      usageResponse={makeGetUsageResponse({
        usageHistory: [
          makeUsageCycle({
            startFormatted: 'Jan 15, 2023',
            endFormatted: 'Feb 14, 2023',
            usage: makeUsage({ igmau: 10, ztamau: 11, tpr: 12, mwi: 13 }),
          }),
          makeUsageCycle({
            startFormatted: 'Feb 15, 2023',
            endFormatted: 'Mar 14, 2023',
            usage: makeUsage({ igmau: 14, ztamau: 15, tpr: 16, mwi: 17 }),
          }),
          makeUsageCycle({
            startFormatted: 'Mar 15, 2023',
            endFormatted: 'Apr 14, 2023',
            usage: makeUsage({ igmau: 18, ztamau: 19, tpr: 20, mwi: 21 }),
          }),
        ],
      })}
    />
  );

  expect(await screen.findByText('Usage History')).toBeInTheDocument();

  // column headers
  expect(screen.getByText('Billing Cycle')).toBeInTheDocument();
  expect(screen.getByText('ZTA MAU')).toBeInTheDocument();
  expect(screen.getByText('ZTA TPR')).toBeInTheDocument();
  expect(screen.getByText('MWI')).toBeInTheDocument();
  expect(screen.getByText('IG MAU')).toBeInTheDocument();
  expect(screen.getByText('IS TPR')).toBeInTheDocument();

  expect(screen.getByText('Jan 15, 2023 - Feb 14, 2023')).toBeInTheDocument();
  expect(screen.getByText('Feb 15, 2023 - Mar 14, 2023')).toBeInTheDocument();
  expect(screen.getByText('Mar 15, 2023 - Apr 14, 2023')).toBeInTheDocument();

  expect(screen.getByText('10')).toBeInTheDocument();
  expect(screen.getByText('11')).toBeInTheDocument();
  expect(screen.getByText('13')).toBeInTheDocument();

  expect(screen.getAllByText('12')).toHaveLength(2);
  expect(screen.getAllByText('16')).toHaveLength(2);
});

test('hides unavailable features', async () => {
  const usageResponse = makeGetUsageResponse({
    missingEntitlements: ['Identity', 'Policy'],
    usageHistory: [makeUsageCycle()],
  });
  render(<UsageHistory usageResponse={usageResponse} />);

  // - populates in 'Identity' and 'Policy' columns
  expect(screen.getAllByText('N/A')).toHaveLength(2);
});

test('shows calibration periods', async () => {
  render(
    <UsageHistory
      usageResponse={makeGetUsageResponse({
        usageHistory: [makeUsageCycle({ calibratingAccounts: 1 })],
      })}
    />
  );

  // for one history row:
  // 5/5 columns show calibrating
  expect(screen.getAllByText('Calibration Period*')).toHaveLength(5);
  expect(screen.getByText(calibrationInfo)).toBeInTheDocument();
});

test('hides calibration periods when a feature is disabled', async () => {
  const usageResponse = makeGetUsageResponse({
    missingEntitlements: ['Policy', 'Identity'],
    usageHistory: [makeUsageCycle({ calibratingAccounts: 1 })],
  });
  render(<UsageHistory usageResponse={usageResponse} />);

  // for one history row:
  // 3/5 columns show calibrating, 2/5 disabled features show -
  expect(screen.getAllByText('Calibration Period*')).toHaveLength(3);
  expect(screen.getAllByText('N/A')).toHaveLength(2);
  expect(screen.getByText(calibrationInfo)).toBeInTheDocument();
});

test('omits calibration explanation if there is no calibration period on table', async () => {
  render(<UsageHistory usageResponse={makeGetUsageResponse()} />);

  expect(screen.queryByText('Calibrating')).not.toBeInTheDocument();
  expect(screen.queryByText(calibrationInfo)).not.toBeInTheDocument();
});
