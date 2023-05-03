import React from 'react';

import { screen } from 'design/utils/testing';

import { InvoiceListProps } from 'e-teleport/Billing/types';
import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { InvoiceList } from 'e-teleport/Billing/PaymentsAndInvoices/InvoiceList';

describe('invoiceList', () => {
  let props: InvoiceListProps;
  const defaultInvoice = {
    invoiceId: 'some-invoiceId',
    status: 'some-status',
    amountDue: 100,
    amountPaid: 0,
    periodEnd: 1682989632,
    periodStart: 1682989632,
    invoicePdf: 'some-invoicePdf',
    usageMau: 55,
  };

  beforeEach(() => {
    props = {
      invoices: [],
      productName: 'some-product-name',
    };
  });

  test('renders table', () => {
    renderWithElementsAndContext(<InvoiceList {...props} />);

    // renders table headers
    expect(screen.getByText('Invoices')).toBeInTheDocument();
    expect(screen.getByText('Date')).toBeInTheDocument();
    expect(screen.getByText('Invoice #')).toBeInTheDocument();
    expect(screen.getByText('Plan')).toBeInTheDocument();
    expect(screen.getByText('Active Users')).toBeInTheDocument();
    expect(screen.getByText('Invoice Total')).toBeInTheDocument();
    expect(screen.getByText('Status')).toBeInTheDocument();
    expect(screen.getByText('Action')).toBeInTheDocument();

    expect(screen.getByText('No Invoices Generated Yet')).toBeInTheDocument();
  });

  test('renders data for each invoice', () => {
    props.invoices = [defaultInvoice, defaultInvoice];
    renderWithElementsAndContext(<InvoiceList {...props} />);

    expect(screen.getAllByText('May 02, 2023')).toHaveLength(2);
    expect(screen.getAllByText('some-invoiceId')).toHaveLength(2);
    expect(screen.getAllByText('some-product-name')).toHaveLength(2);
    expect(screen.getAllByText('$100')).toHaveLength(2);
    expect(screen.getAllByText('55')).toHaveLength(2);
    expect(screen.getAllByText('some-status')).toHaveLength(2);
    expect(screen.getAllByText('Download')).toHaveLength(2);
  });
});
