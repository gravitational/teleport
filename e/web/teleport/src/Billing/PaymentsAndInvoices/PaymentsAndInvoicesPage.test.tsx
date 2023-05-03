import React from 'react';

import { screen } from 'design/utils/testing';

import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { PaymentsAndInvoicesPage } from 'e-teleport/Billing/PaymentsAndInvoices/PaymentsAndInvoicesPage';
import { PaymentsAndInvoicesProps } from 'e-teleport/Billing/types';

describe('paymentsAndInvoicesPage', () => {
  let props: PaymentsAndInvoicesProps;
  beforeEach(() => {
    props = {
      data: {
        usageBasedBilling: true,
        stripePublicKey: 'some-stripePublicKey',
        stripeCustomerId: 'some-stripeCustomerId',
        stripeMissingPaymentMethod: false,
        stripeCardsList: [],
        stripeInvoicesList: [],
        productName: 'some-productName',
        stripeTrialEnd: 0,
        stripeDefaultSourceId: 'some-stripeDefaultSourceId',
      },
      reload: jest.fn(),
    };
  });

  test('if not usage based, does not render', () => {
    props.data.usageBasedBilling = false;
    const { container } = renderWithElementsAndContext(
      <PaymentsAndInvoicesPage {...props} />
    );

    expect(container).toBeEmptyDOMElement();
  });

  test('renders payment CTA if missing payment method', () => {
    props.data.stripeMissingPaymentMethod = true;
    renderWithElementsAndContext(<PaymentsAndInvoicesPage {...props} />);

    expect(screen.getByText('Payment Method')).toBeInTheDocument();
  });

  test('does not render payment CTA if payment method has been added', () => {
    props.data.stripeMissingPaymentMethod = false;
    renderWithElementsAndContext(<PaymentsAndInvoicesPage {...props} />);

    expect(screen.queryByText('Payment Method')).not.toBeInTheDocument();
  });

  test('renders card list if present', () => {
    props.data.stripeMissingPaymentMethod = false;
    props.data.stripeCardsList = [
      {
        id: 'some-id',
        last4: '9999',
        addressLine1: 'some-addressLine1',
        addressLine2: 'some-addressLine2',
        city: 'some-city',
        country: 'some-country',
        state: 'some-state',
        name: 'some-name',
        zip: 'some-zip',
        brand: 'some-brand',
        expirationMonth: 12,
        expirationYear: 2023,
        createdAt: 1682989632,
      },
    ];
    renderWithElementsAndContext(<PaymentsAndInvoicesPage {...props} />);

    expect(screen.getByText('Payment Methods')).toBeInTheDocument();
  });

  test('does not render card list if not present', () => {
    props.data.stripeMissingPaymentMethod = false;
    props.data.stripeCardsList = [];
    renderWithElementsAndContext(<PaymentsAndInvoicesPage {...props} />);

    expect(screen.queryByText('Payment Methods')).not.toBeInTheDocument();
  });

  test('renders invoices if present', () => {
    props.data.stripeInvoicesList = [
      {
        invoiceId: 'some-invoiceId',
        status: 'some-status',
        amountDue: 100,
        amountPaid: 0,
        periodEnd: 1682989632,
        periodStart: 1682989632,
        invoicePdf: 'some-invoicePdf',
        usageMau: 55,
      },
    ];
    renderWithElementsAndContext(<PaymentsAndInvoicesPage {...props} />);

    expect(screen.getByText('Invoices')).toBeInTheDocument();
    expect(
      screen.queryByText('No Invoices Generated Yet ')
    ).not.toBeInTheDocument();
  });

  test('renders empty invoices if no invoices are present', () => {
    props.data.stripeInvoicesList = [];
    renderWithElementsAndContext(<PaymentsAndInvoicesPage {...props} />);

    expect(screen.queryByText('Invoices')).not.toBeInTheDocument();
    expect(screen.getByText('No Invoices Generated Yet')).toBeInTheDocument();
  });
});
