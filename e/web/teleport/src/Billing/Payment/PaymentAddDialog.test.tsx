import React from 'react';

import { screen } from 'design/utils/testing';

import { PaymentAddDialogProps } from 'e-teleport/Billing/types';
import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { PaymentAddDialog } from 'e-teleport/Billing/Payment/PaymentAddDialog';

describe('paymentAddDialog', () => {
  let props: PaymentAddDialogProps;

  beforeEach(() => {
    props = {
      open: true,
      setOpen: jest.fn(),
      title: 'some-title',
    };
  });

  test('displays the title prop', () => {
    props.title = 'A unique title';

    renderWithElementsAndContext(<PaymentAddDialog {...props} />);

    expect(screen.getByText('A unique title')).toBeInTheDocument();
    expect(screen.getByText('Credit Card')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Save And Close' })
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
  });

  test('displays the optional description prop', () => {
    props.description = 'A very original description';

    renderWithElementsAndContext(<PaymentAddDialog {...props} />);

    expect(screen.getByText('A very original description')).toBeInTheDocument();
  });

  test('does not display Make Default Payment if false', () => {
    props.showDefaultOption = false;

    renderWithElementsAndContext(<PaymentAddDialog {...props} />);

    expect(screen.queryByText('Make Default Payment')).not.toBeInTheDocument();
  });

  test('displays Make Default Payment if true', () => {
    props.showDefaultOption = true;

    renderWithElementsAndContext(<PaymentAddDialog {...props} />);

    expect(screen.getByText('Make Default Payment')).toBeInTheDocument();
  });
});
