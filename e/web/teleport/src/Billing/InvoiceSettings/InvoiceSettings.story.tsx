import React from 'react';

import { ContextProvider } from 'teleport';

import { createTeleportContext } from 'teleport/mocks/contexts';

import { MemoryRouter } from 'react-router';

import { Elements } from '@stripe/react-stripe-js';
import { loadStripe } from '@stripe/stripe-js';

import { InvoiceSettingsProps } from 'e-teleport/Billing/types';
import { StripeSubscriptionStatus } from 'e-teleport/Billing/StripeLoader/types';
import { InvoiceSettingsPage } from 'e-teleport/Billing/InvoiceSettings/InvoiceSettingsPage';

export default {
  title: 'TeleportE/Billing',
};

const ctx = createTeleportContext();
// Intentionally not loading Stripe
// Impact: The address form in Invoice Billing Address will not render
// If you want to view the Stripe element, you can find a dev key in the Dev instance of Stripe
const stripePromise = loadStripe('some-stripePublicKey');

export function InvoiceSettingsView() {
  const props: InvoiceSettingsProps = {
    data: {
      usageBasedBilling: true,
      stripePublicKey: 'some-stripePublicKey',
      stripeCustomerId: 'some-stripeCustomerId',
      stripeSubscriptionStatus: StripeSubscriptionStatus.ACTIVE,
      stripeInvoiceEmail: 'hello@example.com',
      stripeInvoicePurchaseOrderNumber: '999',
      stripeCustomerName: 'Jane Doe',
    },
    reload: () => {},
  };

  return (
    <Elements stripe={stripePromise}>
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <InvoiceSettingsPage {...props} />
        </ContextProvider>
      </MemoryRouter>
    </Elements>
  );
}
