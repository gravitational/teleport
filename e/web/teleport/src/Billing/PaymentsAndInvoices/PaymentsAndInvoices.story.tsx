import React from 'react';

import { ContextProvider } from 'teleport';

import { createTeleportContext } from 'teleport/mocks/contexts';

import { MemoryRouter } from 'react-router';

import { Elements } from '@stripe/react-stripe-js';
import { loadStripe } from '@stripe/stripe-js';

import { PaymentsAndInvoicesProps } from 'e-teleport/Billing/types';
import { StripeSubscriptionStatus } from 'e-teleport/Billing/StripeLoader/types';
import { PaymentsAndInvoicesPage } from 'e-teleport/Billing/PaymentsAndInvoices/PaymentsAndInvoicesPage';

export default {
  title: 'TeleportE/Billing',
};

const ctx = createTeleportContext();
// Intentionally not loading Stripe
// Impact: The credit card form in NoPaymentsAndNoInvoicesView will not render if you click "Add a payment method" from the banner
// If you want to view the Stripe element, you can find a dev key in the Dev instance of Stripe
const stripePromise = loadStripe('some-stripePublicKey');

export function PaymentsAndInvoicesView() {
  const props: PaymentsAndInvoicesProps = {
    data: {
      usageBasedBilling: true,
      stripePublicKey: 'some-stripePublicKey',
      stripeCustomerId: 'some-stripeCustomerId',
      stripeSubscriptionStatus: StripeSubscriptionStatus.ACTIVE,

      stripeMissingPaymentMethod: false,
      stripeCardsList: [
        {
          id: '',
          last4: '4455',
          addressLine1: '12 happy street',
          addressLine2: 'apt B',
          city: 'Sunny',
          country: 'OOO',
          state: 'YU',
          name: 'Jane Doe',
          zip: '78788',
          brand: 'Visa',
          expirationMonth: 12,
          expirationYear: 23,
          createdAt: 0,
        },
      ],
      stripeInvoicesList: [
        {
          invoiceId: '777',
          status: 'paid',
          amountDue: 9000,
          amountPaid: 9000,
          periodEnd: 0,
          periodStart: 0,
          invoicePdf: '',
          usageMau: 4,
        },
      ],
      productName: 'Team',
      stripeTrialEnd: 0,
      stripeDefaultSourceId: undefined,
      stripeSubscriptionCancelAt: 0,
      stripeSubscriptionCanceledAt: 0,
    },
    reload: () => {},
  };

  return (
    <Elements stripe={stripePromise}>
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <PaymentsAndInvoicesPage {...props} />
        </ContextProvider>
      </MemoryRouter>
    </Elements>
  );
}

export function NoPaymentsAndNoInvoicesView() {
  const props: PaymentsAndInvoicesProps = {
    data: {
      usageBasedBilling: true,
      stripePublicKey: 'some-stripePublicKey',
      stripeCustomerId: 'some-stripeCustomerId',
      stripeSubscriptionStatus: StripeSubscriptionStatus.ACTIVE,

      stripeMissingPaymentMethod: true,
      stripeCardsList: [],
      stripeInvoicesList: [],
      productName: 'Team',
      stripeTrialEnd: 0,
      stripeDefaultSourceId: undefined,
      stripeSubscriptionCancelAt: 0,
      stripeSubscriptionCanceledAt: 0,
    },
    reload: () => {},
  };

  return (
    <Elements stripe={stripePromise}>
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <PaymentsAndInvoicesPage {...props} />
        </ContextProvider>
      </MemoryRouter>
    </Elements>
  );
}
