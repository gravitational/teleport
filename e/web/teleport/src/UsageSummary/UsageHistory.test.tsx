import { format, getUnixTime } from 'date-fns';

import { render, screen, theme } from 'design/utils/testing';

import { UsageHistoryItem } from 'e-teleport/services/cloud/v1/tenants_pb';

import { usageHistory } from './fixtures';
import { UsageHistory } from './UsageHistory';

test('renders all elements', async () => {
  render(<UsageHistory history={usageHistory} maxMau={300} maxTpr={50} />);

  // title and info text
  expect(await screen.findByText('Usage History')).toBeInTheDocument();
  expect(
    screen.getByText(
      /Your current contract limit is set for 300 MAU and 50 TPR./
    )
  ).toBeInTheDocument();

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

test('shows empty state when there is no usage history', async () => {
  render(<UsageHistory history={[]} maxMau={200} maxTpr={10} />);

  expect(
    await screen.findByText('No cycle information available')
  ).toBeInTheDocument();
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
  };
  render(
    <UsageHistory
      history={[currentCycle, ...usageHistory]}
      maxMau={300}
      maxTpr={50}
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
