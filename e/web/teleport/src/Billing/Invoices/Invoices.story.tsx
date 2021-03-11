import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { Invoices } from './Invoices';
import { makeListItems } from './useInvoices';
import * as fixtures from './../fixtures';

export default {
  title: 'TeleportE/Billing/Invoices',
};

export const Processing = () => {
  return <Invoices {...defaultProps} attempt={{ status: 'processing' }} />;
};

export const Loaded = () => {
  return (
    <MemoryRouter>
      <Invoices {...defaultProps} />
    </MemoryRouter>
  );
};

export const Failed = () => {
  return (
    <Invoices
      {...defaultProps}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  );
};

const defaultProps = {
  attempt: {
    status: 'success' as any,
  },
  invoices: makeListItems(fixtures.invoices),
};
