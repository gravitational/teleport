import React from 'react';
import { fireEvent, screen, userEvent } from 'design/utils/testing';

import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { PurchaseOrder } from 'e-teleport/Billing/InvoiceSettings/PurchaseOrder';

describe('purchaseOrder', () => {
  test('renders', () => {
    renderWithElementsAndContext(<PurchaseOrder />);
    expect(screen.getByText('Invoice Purchase Order')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Optional: Add a purchase order below if you would like one to show up on your invoices.'
      )
    ).toBeInTheDocument();
    expect(screen.getByLabelText('purchase order')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
  });

  test('shows validation error on save', async () => {
    renderWithElementsAndContext(<PurchaseOrder />);
    const input = screen.getByLabelText('purchase order');

    fireEvent.change(input, { target: { value: 'test.com' } });
    expect(input).toHaveValue('test.com');
    expect(
      screen.queryByText(
        'Purchase order format is invalid. Prefix must be 3–12 letters or numbers.'
      )
    ).not.toBeInTheDocument();

    userEvent.click(screen.getByRole('button', { name: 'Save' }));
    await screen.findByText(
      'Purchase order format is invalid. Prefix must be 3–12 letters or numbers.'
    );
  });
});
