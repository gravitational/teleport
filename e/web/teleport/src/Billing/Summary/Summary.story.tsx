import React from 'react';

import { ContextProvider } from 'teleport';

import { createTeleportContext } from 'teleport/mocks/contexts';

import { MemoryRouter } from 'react-router';

import { SummaryProps } from 'e-teleport/Billing/types';
import { SummaryPage } from 'e-teleport/Billing/Summary/SummaryPage';
import { StripeSubscriptionStatus } from 'e-teleport/Billing/StripeLoader/types';
import { StripeUsage } from 'e-teleport/services/cloud';

export default {
  title: 'TeleportE/Billing',
};

const ctx = createTeleportContext();
const defaultUsage: StripeUsage = {
  invoiceId: 'some-invoiceId',
  status: 'some-status',
  periodEnd: 0,
  periodStart: 0,
  usageMau: 31,
  usageTia: 10000,
  usagePr: 51,
};

export function SummaryPageView() {
  const props: SummaryProps = {
    data: {
      usageBasedBilling: true,
      stripePublicKey: 'some-stripePublicKey',
      stripeCustomerId: 'some-stripeCustomerId',
      stripeTrial: false,
      stripeTrialEnd: 1682989632,
      stripeMissingPaymentMethod: false,
      productName: 'Team',
      stripeSubscriptionStatus: StripeSubscriptionStatus.ACTIVE,
      stripeCurrentUsage: defaultUsage,
      stripeSubscriptionCancelAt: 0,
      stripeSubscriptionCanceledAt: 0,
      usageUpdatedAt: 1682900000,
    },
    nonBillableUsage: {
      trustedDeviceUsage: {
        devicesUsageLimit: 5,
        devicesInUse: 4,
      },
      accessRequestUsage: {
        monthlyLimit: 5,
        monthlyUsed: 5,
      },
    },
    reload: () => {},
  };

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <SummaryPage {...props} />
      </ContextProvider>
    </MemoryRouter>
  );
}
