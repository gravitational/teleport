import React from 'react';

import { screen } from 'design/utils/testing';

import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import {
  SummaryPage,
  SummaryProps,
} from 'e-teleport/Billing/EubpSummary/SummaryPage';
import { StripeSubscriptionStatus } from 'e-teleport/Billing/StripeLoader/types';

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
    usageTia: 0,
    usagePr: 0,
  };

  beforeEach(() => {
    props = {
      data: {
        usageBasedBilling: true,
        stripePublicKey: 'some-stripePublicKey',
        stripeCustomerId: 'some-stripeCustomerId',
        stripeTrial: false,
        stripeTrialEnd: 1682989632,
        stripeMissingPaymentMethod: false,
        productName: 'some-productName',
        stripeSubscriptionStatus: StripeSubscriptionStatus.ACTIVE,
        stripeCurrentUsage: defaultUsage,
        stripeSubscriptionCancelAt: 0,
        stripeSubscriptionCanceledAt: 0,
        usageUpdatedAt: 0,
        usageQuota: {
          mauMax: 2,
          tprMax: 20,
          tiaMax: 20000,
          mauInc: 2,
          tprInc: 20,
          tiaInc: 20000,
        },
      },
    };
  });

  test('renders cycle if cycle usage is present', () => {
    renderWithElementsAndContext(<SummaryPage {...props} />);

    expect(screen.getByText(/Current Cycle:/i)).toBeInTheDocument();
  });

  test('does not render cycle if cycle usage is not present', () => {
    props.data.stripeCurrentUsage = null;
    renderWithElementsAndContext(<SummaryPage {...props} />);

    expect(screen.queryByText(/Current Cycle/i)).not.toBeInTheDocument();
    expect(
      screen.getByText(/Usage data is being gathered./i)
    ).toBeInTheDocument();
  });
});
