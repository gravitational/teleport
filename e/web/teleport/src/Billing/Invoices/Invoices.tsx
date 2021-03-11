import React from 'react';
import { Box, Alert, Indicator } from 'design';
import useTeleportE from 'e-teleport/useTeleportE';
import useInvoices, { State } from './useInvoices';
import InvoiceList from './InvoiceList';

export default function Container() {
  const ctx = useTeleportE();
  const state = useInvoices(ctx);

  return <Invoices {...state} />;
}

export function Invoices({ attempt, invoices }: State) {
  return (
    <>
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'failed' && (
        <Alert kind="danger" children={attempt.statusText} />
      )}
      {attempt.status === 'success' && <InvoiceList invoices={invoices} />}
    </>
  );
}
