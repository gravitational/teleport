import React from 'react';

import { screen } from 'design/utils/testing';

import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { SummaryProps } from 'e-teleport/Billing/types';
import { SummaryPage } from 'e-teleport/Billing/Summary/SummaryPage';
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
      },
      nonBillableUsage: {
        trustedDeviceUsage: {
          devicesInUse: 0,
          devicesUsageLimit: 0,
        },
        accessRequestUsage: {
          monthlyLimit: 0,
          monthlyUsed: 0,
        },
      },
      reload: jest.fn(),
    };
  });

  test('renders payment CTA if missing payment method', () => {
    props.data.stripeMissingPaymentMethod = true;
    renderWithElementsAndContext(<SummaryPage {...props} />);

    expect(screen.getByText('Payment Method')).toBeInTheDocument();
  });

  test('does not render payment CTA if payment method has been added', () => {
    props.data.stripeMissingPaymentMethod = false;
    renderWithElementsAndContext(<SummaryPage {...props} />);

    expect(screen.queryByText('Payment Method')).not.toBeInTheDocument();
  });

  test('renders cycle if cycle usage is present', () => {
    renderWithElementsAndContext(<SummaryPage {...props} />);

    expect(screen.getByText(/Current Cycle:/i)).toBeInTheDocument();
  });

  test('does not render cycle if cycle usage is not present', () => {
    props.data.stripeCurrentUsage = null;
    renderWithElementsAndContext(<SummaryPage {...props} />);

    expect(screen.queryByText(/Current Cycle/i)).not.toBeInTheDocument();
  });

  test('renders cancel banner', () => {
    renderWithElementsAndContext(<SummaryPage {...props} />);

    expect(
      screen.getByRole('button', { name: 'Cancel Plan' })
    ).toBeInTheDocument();
  });
});
