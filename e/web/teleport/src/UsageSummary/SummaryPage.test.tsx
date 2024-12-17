import { render, screen } from 'design/utils/testing';

import { SummaryPage } from 'e-teleport/UsageSummary/SummaryPage';
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
