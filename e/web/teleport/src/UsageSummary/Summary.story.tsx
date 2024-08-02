import React from 'react';

import { ContextProvider } from 'teleport';

import { createTeleportContext } from 'teleport/mocks/contexts';

import { MemoryRouter } from 'react-router';

import { StripeUsage } from 'e-teleport/services/cloud';
import { SummaryPage, SummaryProps } from 'e-teleport/UsageSummary/SummaryPage';

export default {
  title: 'TeleportE/Billing/Enterprise Usage-Based',
};

const ctx = createTeleportContext();
const defaultUsage: StripeUsage = {
  invoiceId: 'some-invoiceId',
  status: 'some-status',
  periodEnd: 0,
  periodStart: 0,
  usageMau: 31,
  usagePr: 351,
};

const defaultProps = (): SummaryProps => ({
  data: {
    usageBasedBilling: true,
    stripePublicKey: 'some-stripePublicKey',
    stripeCustomerId: 'some-stripeCustomerId',
    stripeTrial: false,
    stripeTrialEnd: 1682989632,
    stripeMissingPaymentMethod: false,
    productName: 'Team',
    stripeSubscriptionStatus: null,
    stripeCurrentUsage: defaultUsage,
    stripeSubscriptionCancelAt: 0,
    stripeSubscriptionCanceledAt: 0,
    usageUpdatedAt: 1682900000,
    usageQuota: {
      mauMax: 30,
      mauInc: 30,
      tprMax: 500,
      tprInc: 500,
    },
  },
});

export function SummaryPageView() {
  const props = defaultProps();

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <SummaryPage {...props} />
      </ContextProvider>
    </MemoryRouter>
  );
}

export function EmptySummaryPageView() {
  const props = defaultProps();
  props.data.stripeCurrentUsage = null;

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <SummaryPage {...props} />
      </ContextProvider>
    </MemoryRouter>
  );
}
