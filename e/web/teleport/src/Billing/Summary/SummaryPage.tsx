import React from 'react';

import { Cycle } from 'e-teleport/Billing/Summary/Cycle';
import { PaymentBanner } from 'e-teleport/Billing/Payment/PaymentBanner';
import { StatusBanner } from 'e-teleport/Billing/common/StatusBanner';
import {
  BillingSummaryInformation,
  NonBillableSummaryInformation,
} from 'e-teleport/services/cloud';
export interface SummaryProps {
  data: BillingSummaryInformation;
  nonBillableUsage: NonBillableSummaryInformation;
  reload: () => void;
}

export const SummaryPage = ({
  data: {
    productName,
    stripeCurrentUsage,
    stripeMissingPaymentMethod,
    stripeSubscriptionStatus,
    stripeTrialEnd,
    stripeSubscriptionCanceledAt,
    usageUpdatedAt,
  },
  nonBillableUsage,
  reload,
}: SummaryProps) => (
  <>
    {stripeMissingPaymentMethod && (
      <PaymentBanner
        productName={productName}
        reload={reload}
        stripeTrialEnd={stripeTrialEnd}
      />
    )}
    {stripeCurrentUsage && (
      <Cycle
        currentUsage={stripeCurrentUsage}
        nonBillableUsage={nonBillableUsage}
        productName={productName}
        stripeMissingPaymentMethod={stripeMissingPaymentMethod}
        stripeTrialEnd={stripeTrialEnd}
        usageUpdatedAt={usageUpdatedAt}
      />
    )}
    <StatusBanner
      productName={productName}
      stripeCurrentPeriodEnd={
        stripeCurrentUsage ? stripeCurrentUsage.periodEnd : 0
      }
      stripeMissingPaymentMethod={stripeMissingPaymentMethod}
      stripeSubscriptionStatus={stripeSubscriptionStatus}
      stripeTrialEnd={stripeTrialEnd}
      stripeSubscriptionCanceled={stripeSubscriptionCanceledAt != 0}
    />
  </>
);
