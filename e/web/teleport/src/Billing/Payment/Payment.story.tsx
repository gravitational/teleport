import React from 'react';

import { ContextProvider } from 'teleport';

import { createTeleportContext } from 'teleport/mocks/contexts';

import { MemoryRouter } from 'react-router';

import { Elements } from '@stripe/react-stripe-js';
import { loadStripe } from '@stripe/stripe-js';

import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { PaymentDeleteDialog } from 'e-teleport/Billing/Payment/PaymentDeleteDialog';
import { PaymentAddDialog } from 'e-teleport/Billing/Payment/PaymentAddDialog';
import {
  ExistingPaymentProps,
  PaymentAddDialogProps,
} from 'e-teleport/Billing/types';

export default {
  title: 'TeleportE/Billing',
};

const ctx = createTeleportContext();
// Intentionally not loading Stripe
// Impact: The credit card form in PaymentAddDialogView will not render
// If you want to view the Stripe element, you can find a dev key in the Dev instance of Stripe
const stripePromise = loadStripe('some-stripePublicKey');

export function PaymentAddDialogView() {
  const props: PaymentAddDialogProps = {
    open: true,
    setOpen: () => {},
    title: 'Custom Title',
    stripeMissingPaymentMethod: false,
    description: 'custom description',
    makeDefault: true,
    showDefaultOption: true,
  };

  return (
    <MemoryRouter>
      <TeleportContextProvider ctx={ctx}>
        <Elements stripe={stripePromise}>
          <PaymentAddDialog {...props} />
        </Elements>
      </TeleportContextProvider>
    </MemoryRouter>
  );
}

export function PaymentDeleteDialogView() {
  const props: ExistingPaymentProps = {
    open: true,
    setOpen: () => {},
    card: {
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
  };

  return (
    <Elements stripe={stripePromise}>
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <PaymentDeleteDialog {...props} />
        </ContextProvider>
      </MemoryRouter>
    </Elements>
  );
}
