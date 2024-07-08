import React from 'react';
import { fireEvent, screen, userEvent } from 'design/utils/testing';

import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { PurchaseOrder } from 'e-teleport/Billing/InvoiceSettings/PurchaseOrder';
import { PurchaseOrderProps } from 'e-teleport/Billing/types';

describe('purchaseOrder', () => {
  let props: PurchaseOrderProps;

  beforeEach(() => {
    props = {
      po: '999',
      reload: jest.fn(),
    };
  });

  test('renders', () => {
    renderWithElementsAndContext(<PurchaseOrder {...props} />);
    expect(screen.getByText('Invoice Purchase Order')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Optional: Add a purchase order below if you would like one to show up on your invoices.'
      )
    ).toBeInTheDocument();
    expect(screen.getByLabelText('purchase order')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
  });

  test('enables save when field updates', () => {
    props.po = '99999';
    renderWithElementsAndContext(<PurchaseOrder {...props} />);
    expect(screen.getByText('Invoice Purchase Order')).toBeInTheDocument();

    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();

    const input = screen.getByLabelText('purchase order');
    expect(input).toHaveValue('99999');
    fireEvent.change(input, { target: { value: '888888' } });
    expect(input).toHaveValue('888888');

    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled();
  });

  test('shows validation error on save', async () => {
    renderWithElementsAndContext(<PurchaseOrder {...props} />);
    const input = screen.getByLabelText('purchase order');

    fireEvent.change(input, { target: { value: 'test.com' } });
    expect(input).toHaveValue('test.com');
    expect(
      screen.queryByText(
        'Purchase order format is invalid. Prefix must be 3–12 letters or numbers.'
      )
    ).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Save' }));
    await screen.findByText(
      'Purchase order format is invalid. Prefix must be 3–12 letters or numbers.'
    );
  });
});
