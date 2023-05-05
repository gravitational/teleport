import React from 'react';

import { screen } from 'design/utils/testing';

import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { InvoiceSettingsPage } from 'e-teleport/Billing/InvoiceSettings/InvoiceSettingsPage';
import { InvoiceSettingsProps } from 'e-teleport/Billing/types';

describe('invoiceSettingsPage', () => {
  let props: InvoiceSettingsProps;
  beforeEach(() => {
    props = {
      data: {
        usageBasedBilling: true,
        stripePublicKey: 'some-stripePublicKey',
        stripeCustomerId: 'some-stripeCustomerId',
        stripeInvoiceEmail: 'some-stripeInvoiceEmail',
        stripeInvoicePurchaseOrderNumber:
          'some-stripeInvoicePurchaseOrderNumber',
        stripeCustomerName: 'some-stripeCustomerName',
      },
      reload: jest.fn(),
    };
  });

  test('if not usage based, does not render', () => {
    props.data.usageBasedBilling = false;
    const { container } = renderWithElementsAndContext(
      <InvoiceSettingsPage {...props} />
    );

    expect(container).toBeEmptyDOMElement();
  });

  test('renders address, email and purchase order', () => {
    renderWithElementsAndContext(<InvoiceSettingsPage {...props} />);

    expect(screen.getByText('Invoice Billing Address')).toBeInTheDocument();
    expect(screen.getByText('Invoice Email Recipient')).toBeInTheDocument();
    expect(screen.getByText('Invoice Purchase Order')).toBeInTheDocument();
  });
});
