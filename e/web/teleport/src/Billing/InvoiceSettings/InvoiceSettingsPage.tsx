import React from 'react';

import { PurchaseOrder } from 'e-teleport/Billing/InvoiceSettings/PurchaseOrder';
import { Address } from 'e-teleport/Billing/InvoiceSettings/Address';
import { Email } from 'e-teleport/Billing/InvoiceSettings/Email';

import { InvoiceSettingsProps } from 'e-teleport/Billing/types';
import { StripeSubscriptionStatus } from 'e-teleport/Billing/StripeLoader/types';
import { CanceledBanner } from 'e-teleport/Billing/common/CanceledBanner';

export const InvoiceSettingsPage = ({
  data: {
    stripeInvoiceBillingAddress,
    stripeInvoiceEmail,
    stripeInvoicePurchaseOrderNumber,
    stripeCustomerName,
    stripeSubscriptionStatus,
  },
}: InvoiceSettingsProps) => (
  <>
    {stripeSubscriptionStatus === StripeSubscriptionStatus.CANCELED && (
      <CanceledBanner />
    )}
    <Address address={stripeInvoiceBillingAddress} name={stripeCustomerName} />
    <Email email={stripeInvoiceEmail} />
    <PurchaseOrder po={stripeInvoicePurchaseOrderNumber} />
  </>
);
