import React from 'react';

import { Cycle } from 'e-teleport/Billing/Summary/Cycle';
import { SummaryProps } from 'e-teleport/Billing/types';
import { PaymentBanner } from 'e-teleport/Billing/Payment/PaymentBanner';
import { CancelBanner } from 'e-teleport/Billing/common/CancelBanner';

export const SummaryPage = ({
  data: {
    stripeMissingPaymentMethod,
    productName,
    stripeTrialEnd,
    stripeCurrentUsage,
    usageBasedBilling,
  },
  reload,
}: SummaryProps) => {
  // todo (michellescripts) this should be filtered out at the Features.tsx level as part of https://github.com/gravitational/cloud/issues/3536
  if (!usageBasedBilling) {
    return null;
  }

  return (
    <>
      {stripeMissingPaymentMethod && (
        <PaymentBanner
          productName={productName}
          reload={reload}
          stripeTrialEnd={stripeTrialEnd}
        />
      )}
      {stripeCurrentUsage && <Cycle currentUsage={stripeCurrentUsage} />}
      <CancelBanner
        productName={productName}
        stripeMissingPaymentMethod={stripeMissingPaymentMethod}
        stripeTrialEnd={stripeTrialEnd}
      />
    </>
  );
};
