import React from 'react';
import { fireEvent, screen, userEvent } from 'design/utils/testing';

import { renderWithElementsAndContext } from 'e-teleport/Billing/StripeLoader/testhelper/renderWithElementsAndContext';
import { Email } from 'e-teleport/Billing/InvoiceSettings/Email';
import { EmailProps } from 'e-teleport/Billing/types';

describe('email', () => {
  let props: EmailProps;

  beforeEach(() => {
    props = {
      email: 'test@example.com',
      reload: jest.fn(),
    };
  });

  test('renders with props', () => {
    renderWithElementsAndContext(<Email {...props} />);
    expect(screen.getByText('Invoice Email Recipient')).toBeInTheDocument();
    expect(
      screen.getByText(
        "Optional: Invoices will be sent to the email address of the cluster's creator by default. Add an address here if you want invoices to go to a different address, instead:"
      )
    ).toBeInTheDocument();

    const input = screen.getByLabelText('email');
    expect(input).toHaveValue('test@example.com');
    expect(screen.getByLabelText('email')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
  });

  test('enables save when field updates', () => {
    renderWithElementsAndContext(<Email {...props} />);
    expect(screen.getByText('Invoice Email Recipient')).toBeInTheDocument();

    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();

    const input = screen.getByLabelText('email');
    expect(input).toHaveValue('test@example.com');
    fireEvent.change(input, { target: { value: 'new-email@hey.com' } });
    expect(input).toHaveValue('new-email@hey.com');

    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled();
  });

  test('shows validation error on save', async () => {
    renderWithElementsAndContext(<Email {...props} />);
    const input = screen.getByLabelText('email');

    fireEvent.change(input, { target: { value: 'test.com' } });
    expect(input).toHaveValue('test.com');
    expect(
      screen.queryByText('Email format is invalid')
    ).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Save' }));
    await screen.findByText('Email format is invalid');
  });
});
