import { screen } from 'design/utils/testing';

import React from 'react';

import { ExistingPaymentProps } from 'e-teleport/Billing/types';
import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { PaymentDeleteDialog } from 'e-teleport/Billing/Payment/PaymentDeleteDialog';

describe('paymentDeleteDialog', () => {
  let props: ExistingPaymentProps;
  const defaultCard = {
    id: 'some-id',
    last4: 'some-last4',
    addressLine1: 'some-addressLine1',
    addressLine2: 'some-addressLine2',
    city: 'some-city',
    country: 'some-country',
    state: 'some-state',
    name: 'some-name',
    zip: 'some-zip',
    brand: 'some-brand',
    expirationMonth: 1682989632,
    expirationYear: 2023,
    createdAt: 1682989632,
  };

  beforeEach(() => {
    props = {
      open: true,
      setOpen: jest.fn(),
      card: defaultCard,
    };
  });

  test('displays delete dialog', () => {
    props.card.brand = 'VISA';
    props.card.last4 = '9999';
    renderWithElementsAndContext(<PaymentDeleteDialog {...props} />);

    expect(
      screen.getByText(/Are you sure you want to delete this payment method?/i)
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /You are about to delete VISA 9999 from your account. Once removed, it will be unavailable for use./i
      )
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Delete Card' })
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
  });
});
