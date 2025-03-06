import { format, getUnixTime } from 'date-fns';

import { render, screen, theme } from 'design/utils/testing';

import { UsageHistoryItem } from 'e-teleport/services/cloud/v1/tenants_pb';

import { usageHistory } from './fixtures';
import { UsageHistory } from './UsageHistory';

test('shows empty state when there is no usage history', async () => {
  render(
    <UsageHistory
      history={[]}
      hasCloudAnonymizationKey={false}
      salesforceIdUpdatedAt={0}
    />
  );

  expect(
    await screen.findByText('No cycle information available')
  ).toBeInTheDocument();
});

test('renders all elements', async () => {
  render(
    <UsageHistory
      history={usageHistory}
      hasCloudAnonymizationKey={false}
      salesforceIdUpdatedAt={0}
    />
  );

  expect(await screen.findByText('Usage History')).toBeInTheDocument();

  // column headers
  expect(screen.getByText('Billing Cycle')).toBeInTheDocument();
  expect(screen.getByText('Monthly Active Users (MAU)')).toBeInTheDocument();
  expect(
    screen.getByText('Teleport Protected Resources (TPR)')
  ).toBeInTheDocument();

  // rows
  expect(screen.getByText('Jan 15, 2023 - Feb 14, 2023')).toBeInTheDocument();
  expect(screen.getByText('100')).toBeInTheDocument();
  expect(screen.getByText('1000')).toBeInTheDocument();

  expect(screen.getByText('Feb 15, 2023 - Mar 14, 2023')).toBeInTheDocument();
  expect(screen.getByText('101')).toBeInTheDocument();
  expect(screen.getByText('1001')).toBeInTheDocument();

  expect(screen.getByText('Mar 15, 2023 - Apr 14, 2023')).toBeInTheDocument();
  expect(screen.getByText('102')).toBeInTheDocument();
  expect(screen.getByText('1002')).toBeInTheDocument();
});

test('highlights the current cycle', async () => {
  const now = new Date();
  const currentCycle: UsageHistoryItem = {
    cycleStart: getUnixTime(now),
    cycleStartFormatted: format(now, 'LLL dd, yyyy'),
    cycleEnd: getUnixTime(new Date('2100/01/01')),
    cycleEndFormatted: 'Jan 01, 2100',
    mau: 100,
    tpr: 1000,
    mwi: 100,
    igmau: 10,
  };
  render(
    <UsageHistory
      history={[currentCycle, ...usageHistory]}
      hasCloudAnonymizationKey={false}
      salesforceIdUpdatedAt={0}
    />
  );

  // ignore the header
  const [, firstRow, ...rest] = screen.getAllByRole('row');

  // first row should be the only one highlighted
  [...firstRow.children].forEach(cell => {
    expect(cell).toHaveStyle(
      `background-color: ${theme.colors.levels.surface}`
    );
  });

  rest.forEach(row => {
    [...row.children].forEach(cell => {
      expect(cell).not.toHaveStyle(
        `background-color: ${theme.colors.levels.surface}`
      );
    });
  });
});

test('shows calibration periods', async () => {
  render(
    <UsageHistory
      history={usageHistory}
      hasCloudAnonymizationKey={true}
      salesforceIdUpdatedAt={usageHistory[1].cycleStart + 1}
    />
  );

  expect(screen.queryByText(usageHistory[1].mau)).not.toBeInTheDocument();
  expect(screen.queryByText(usageHistory[1].tpr)).not.toBeInTheDocument();
  expect(screen.getAllByText('Calibration Period')).toHaveLength(2);
});
