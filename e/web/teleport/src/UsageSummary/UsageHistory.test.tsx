import { format, getUnixTime } from 'date-fns';

import { render, screen, theme } from 'design/utils/testing';

import { UsageHistoryItem } from 'e-teleport/services/cloud/v1/tenants_pb';

import { usageHistory } from './fixtures';
import { UsageHistory } from './UsageHistory';

const calibrationInfo =
  /A change to your account required a calibration period in order to accurately count Active Users across trusted clusters/;

test('shows empty state when there is no usage history', async () => {
  render(
    <UsageHistory
      cloud
      history={[]}
      hasCloudAnonymizationKey={false}
      salesforceIdUpdatedAt={0}
      hasIdentityGovernance={true}
      hasIdentitySecurity={true}
    />
  );

  expect(
    await screen.findByText('No cycle information available')
  ).toBeInTheDocument();
});

test('renders all elements', async () => {
  render(
    <UsageHistory
      cloud
      history={usageHistory}
      hasCloudAnonymizationKey={false}
      salesforceIdUpdatedAt={0}
      hasIdentityGovernance={true}
      hasIdentitySecurity={true}
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

  // rows
  expect(screen.getByText('Jan 15, 2023 - Feb 14, 2023')).toBeInTheDocument();
  expect(screen.getByText('100')).toBeInTheDocument();
  expect(screen.getAllByText('1000')).toHaveLength(2);

  expect(screen.getByText('Feb 15, 2023 - Mar 14, 2023')).toBeInTheDocument();
  expect(screen.getByText('101')).toBeInTheDocument();
  expect(screen.getAllByText('1001')).toHaveLength(2);

  expect(screen.getByText('Mar 15, 2023 - Apr 14, 2023')).toBeInTheDocument();
  expect(screen.getByText('102')).toBeInTheDocument();
  expect(screen.getAllByText('1000')).toHaveLength(2);
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
      cloud
      history={[currentCycle, ...usageHistory]}
      hasCloudAnonymizationKey={false}
      salesforceIdUpdatedAt={0}
      hasIdentityGovernance={true}
      hasIdentitySecurity={true}
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

test('hides unavailable features', async () => {
  render(
    <UsageHistory
      cloud
      history={usageHistory}
      hasCloudAnonymizationKey={true}
      salesforceIdUpdatedAt={0}
      hasIdentityGovernance={false}
      hasIdentitySecurity={false}
    />
  );

  expect(screen.queryByText(usageHistory[1].igmau)).not.toBeInTheDocument();
  const qtyDashCells = usageHistory.length * 2; // one for each missing feature per row
  expect(screen.getAllByText('-')).toHaveLength(qtyDashCells);
});

test('shows calibration periods', async () => {
  render(
    <UsageHistory
      cloud
      history={usageHistory}
      hasCloudAnonymizationKey={true}
      salesforceIdUpdatedAt={usageHistory[1].cycleStart + 1}
      hasIdentityGovernance={true}
      hasIdentitySecurity={true}
    />
  );

  expect(screen.queryByText(usageHistory[1].mau)).not.toBeInTheDocument();
  expect(screen.queryByText(usageHistory[1].tpr)).not.toBeInTheDocument();
  expect(screen.getAllByText('Calibration Period*')).toHaveLength(5);
  expect(screen.getByText(calibrationInfo)).toBeInTheDocument();
});

test('hides calibration periods when a feature is disabled', async () => {
  render(
    <UsageHistory
      cloud
      history={usageHistory}
      hasCloudAnonymizationKey={true}
      salesforceIdUpdatedAt={usageHistory[1].cycleStart + 1}
      hasIdentityGovernance={false}
      hasIdentitySecurity={true}
    />
  );

  expect(screen.queryByText(usageHistory[1].mau)).not.toBeInTheDocument();
  expect(screen.queryByText(usageHistory[1].tpr)).not.toBeInTheDocument();
  expect(screen.getAllByText('Calibration Period*')).toHaveLength(4);
  expect(screen.getAllByText('-')).toHaveLength(usageHistory.length); // one for each row
  expect(screen.getByText(calibrationInfo)).toBeInTheDocument();
});

test('hides calibration periods when not cloud', async () => {
  render(
    <UsageHistory
      cloud={false}
      history={usageHistory}
      hasCloudAnonymizationKey={true}
      salesforceIdUpdatedAt={usageHistory[1].cycleStart + 1}
      hasIdentityGovernance={true}
      hasIdentitySecurity={true}
    />
  );

  expect(screen.queryByText('Calibration Period*')).not.toBeInTheDocument();
  expect(screen.queryByText(calibrationInfo)).not.toBeInTheDocument();
});

test('omits calibration explanation if there is no calibration period on table', async () => {
  render(
    <UsageHistory
      cloud
      history={usageHistory}
      hasCloudAnonymizationKey={true}
      salesforceIdUpdatedAt={0}
      hasIdentityGovernance={true}
      hasIdentitySecurity={true}
    />
  );

  expect(screen.queryByText('Calibration Period*')).not.toBeInTheDocument();
  expect(screen.queryByText(calibrationInfo)).not.toBeInTheDocument();
});
