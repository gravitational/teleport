import React from 'react';
import { render, screen } from 'design/utils/testing';

import { SummaryPage, SummaryProps } from 'e-teleport/UsageSummary/SummaryPage';

jest.mock('teleport/useStickyClusterId', () =>
  jest.fn(() => ({ clusterId: 'cluster-name', isLeafCluster: false }))
);

describe('summaryPage', () => {
  let props: SummaryProps;
  const defaultUsage = {
    invoiceId: 'some-invoiceId',
    status: 'some-status',
    periodEnd: 0,
    periodStart: 0,
    usageMau: 0,
    usagePr: 0,
  };

  beforeEach(() => {
    props = {
      data: {
        usageBasedBilling: true,
        productName: 'some-productName',
        stripeCurrentUsage: defaultUsage,
        usageUpdatedAt: 0,
        usageQuota: {
          mauMax: 2,
          tprMax: 20,
          mauInc: 2,
          tprInc: 20,
        },
      },
    };
  });

  test('renders cycle if cycle usage is present', () => {
    render(<SummaryPage {...props} />);

    expect(screen.getByText(/Current Cycle:/i)).toBeInTheDocument();
  });

  test('does not render cycle if cycle usage is not present', () => {
    props.data.stripeCurrentUsage = null;
    render(<SummaryPage {...props} />);

    expect(screen.queryByText(/Current Cycle/i)).not.toBeInTheDocument();
    expect(
      screen.getByText(/Usage data is being gathered./i)
    ).toBeInTheDocument();
  });
});
