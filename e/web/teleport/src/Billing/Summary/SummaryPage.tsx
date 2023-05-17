import React from 'react';

import { Cycle } from 'e-teleport/Billing/Summary/Cycle';
import { SummaryProps } from 'e-teleport/Billing/types';
import { PaymentBanner } from 'e-teleport/Billing/Payment/PaymentBanner';
import { StatusBanner } from 'e-teleport/Billing/common/StatusBanner';
import { StripeSubscriptionStatus } from 'e-teleport/Billing/StripeLoader/types';
import { CanceledBanner } from 'e-teleport/Billing/common/CanceledBanner';

export const SummaryPage = ({
  data: {
    productName,
    stripeCurrentUsage,
    stripeMissingPaymentMethod,
    stripeSubscriptionStatus,
    stripeTrialEnd,
    stripeSubscriptionCancelAt,
    stripeSubscriptionCanceledAt,
  },
  reload,
}: SummaryProps) => (
  <>
    {(stripeSubscriptionStatus === StripeSubscriptionStatus.CANCELED ||
      stripeSubscriptionCanceledAt != 0) && (
      <CanceledBanner stripeSubscriptionCancelAt={stripeSubscriptionCancelAt} />
    )}
    {stripeMissingPaymentMethod && (
      <PaymentBanner
        productName={productName}
        reload={reload}
        stripeTrialEnd={stripeTrialEnd}
      />
    )}
    {stripeCurrentUsage && <Cycle currentUsage={stripeCurrentUsage} />}
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
