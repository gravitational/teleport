import React from 'react';

import { screen, userEvent } from 'design/utils/testing';

import { PaymentBannerProps } from 'e-teleport/Billing/types';
import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { PaymentBanner } from 'e-teleport/Billing/Payment/PaymentBanner';

describe('paymentBanner', () => {
  let props: PaymentBannerProps;

  beforeEach(() => {
    props = {
      productName: '',
      reload: jest.fn(),
      stripeTrialEnd: 1682989632,
    };
  });

  test('renders payment CTA', () => {
    props.productName = 'SuperPro';
    renderWithElementsAndContext(<PaymentBanner {...props} />);

    expect(screen.getByText('Payment Method')).toBeInTheDocument();
    expect(
      screen.getByText(/Required for the Teleport SuperPro Plan/i)
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /Add a default credit card below to automatically upgrade to the SuperPro plan when your trial expires./i
      )
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Add a Payment Method' })
    ).toBeInTheDocument();
  });

  test('clicking CTA opens modal', async () => {
    renderWithElementsAndContext(<PaymentBanner {...props} />);

    expect(screen.queryByTestId('Modal')).not.toBeInTheDocument();
    await userEvent.click(
      screen.getByRole('button', { name: 'Add a Payment Method' })
    );
    expect(screen.getByTestId('Modal')).toBeInTheDocument();
  });
});
