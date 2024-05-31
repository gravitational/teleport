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
    stripeCards,
    stripeInvoices,
    productName,
    stripeTrialEnd,
    stripeDefaultSourceId,
  } = pageState;

  return (
    <>
      {stripeMissingPaymentMethod && (
        <PaymentBanner
          productName={productName}
          reload={reload}
          stripeTrialEnd={stripeTrialEnd}
        />
      )}
      {!stripeMissingPaymentMethod && stripeCards.length > 0 && (
        <CardList
          cards={stripeCards}
          setPageState={setPageState}
          defaultSourceID={stripeDefaultSourceId}
          stripeMissingPaymentMethod={stripeMissingPaymentMethod}
        />
      )}
      {(!stripeInvoices || stripeInvoices.length === 0) && (
        <InvoiceEmptyState />
      )}
      {stripeInvoices && stripeInvoices.length > 0 && (
        <InvoiceList invoices={stripeInvoices} productName={productName} />
      )}
    </>
  );
};
