import React, { useState } from 'react';

import { PaymentsAndInvoicesProps } from 'e-teleport/Billing/types';
import { PaymentBanner } from 'e-teleport/Billing/Payment/PaymentBanner';
import { CardList } from 'e-teleport/Billing/PaymentsAndInvoices/CardList';
import { InvoiceEmptyState } from 'e-teleport/Billing/PaymentsAndInvoices/InvoiceEmptyState';
import { InvoiceList } from 'e-teleport/Billing/PaymentsAndInvoices/InvoiceList';
import { PaymentsInvoicesInformation } from 'e-teleport/services/cloud';

export const PaymentsAndInvoicesPage = ({
  data,
  reload,
}: PaymentsAndInvoicesProps) => {
  const [pageState, setPageState] = useState<PaymentsInvoicesInformation>(data);
  const {
    stripeMissingPaymentMethod,
    stripeCardsList,
    stripeInvoicesList,
    productName,
    stripeTrialEnd,
    stripeDefaultSourceId,
    usageBasedBilling,
  } = pageState;

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
      {!stripeMissingPaymentMethod && stripeCardsList.length > 0 && (
        <CardList
          cards={stripeCardsList}
          setPageState={setPageState}
          defaultSourceID={stripeDefaultSourceId}
        />
      )}
      {(!stripeInvoicesList || stripeInvoicesList.length === 0) && (
        <InvoiceEmptyState />
      )}
      {stripeInvoicesList && stripeInvoicesList.length > 0 && (
        <InvoiceList invoices={stripeInvoicesList} productName={productName} />
      )}
    </>
  );
};
