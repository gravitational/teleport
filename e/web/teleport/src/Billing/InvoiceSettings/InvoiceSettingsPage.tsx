import React from 'react';

import { PurchaseOrder } from 'e-teleport/Billing/InvoiceSettings/PurchaseOrder';
import { Address } from 'e-teleport/Billing/InvoiceSettings/Address';
import { Email } from 'e-teleport/Billing/InvoiceSettings/Email';

import { InvoiceSettingsProps } from 'e-teleport/Billing/types';

export const InvoiceSettingsPage = ({
  data: {
    stripeInvoiceBillingAddress,
    stripeInvoiceEmail,
    stripeInvoicePurchaseOrderNumber,
    stripeCustomerName,
  },
  reload,
}: InvoiceSettingsProps) => (
  <>
    <Address
      address={stripeInvoiceBillingAddress}
      name={stripeCustomerName}
      reload={reload}
    />
    <Email email={stripeInvoiceEmail} reload={reload} />
    <PurchaseOrder po={stripeInvoicePurchaseOrderNumber} reload={reload} />
  </>
);
