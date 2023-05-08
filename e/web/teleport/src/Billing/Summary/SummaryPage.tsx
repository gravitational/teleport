import React from 'react';

import { Cycle } from 'e-teleport/Billing/Summary/Cycle';
import { SummaryProps } from 'e-teleport/Billing/types';
import { PaymentBanner } from 'e-teleport/Billing/Payment/PaymentBanner';
import { StatusBanner } from 'e-teleport/Billing/common/StatusBanner';
import { StripeSubscriptionStatus } from 'e-teleport/Billing/StripeLoader/types';
import { CanceledBanner } from 'e-teleport/Billing/common/CanceledBanner';

export const SummaryPage = ({
  data: {
    stripeMissingPaymentMethod,
    productName,
    stripeTrialEnd,
    stripeCurrentUsage,
    usageBasedBilling,
    stripeSubscriptionStatus,
  },
  reload,
}: SummaryProps) => {
  // todo (michellescripts) this should be filtered out at the Features.tsx level as part of https://github.com/gravitational/cloud/issues/3536
  if (!usageBasedBilling) {
    return null;
  }

  return (
    <>
      {stripeSubscriptionStatus === StripeSubscriptionStatus.CANCELED && (
        <CanceledBanner />
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
        stripeTrialEnd={stripeTrialEnd}
        stripeSubscriptionStatus={stripeSubscriptionStatus}
        stripeMissingPaymentMethod={stripeMissingPaymentMethod}
      />
    </>
  );
};
