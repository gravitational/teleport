import React from 'react';
import { Box, ButtonBorder, Link } from 'design';
import Table, { Cell } from 'design/DataTable';

import { displayUnixShortDate } from 'shared/services/loc/loc';

import { InvoiceListProps } from 'e-teleport/Billing/types';

export const InvoiceList = ({ invoices, productName }: InvoiceListProps) => {
  // todo (michellescripts) add "view" action;  https://github.com/gravitational/cloud/issues/3536
  const renderActionCell = (invoiceURL: string): React.ReactElement => {
    return (
      <Cell>
        <Link href={invoiceURL} target="__blank" color="text.primary">
          <ButtonBorder>Download</ButtonBorder>
        </Link>
      </Cell>
    );
  };

  return (
    <>
      <h3>Invoices</h3>
      <Box m="20px 0 0 0">
        <Table
          data={invoices.map(invoice => ({
            ...invoice,
            productName: productName,
          }))}
          columns={[
            {
              key: 'periodEnd',
              headerText: 'Date',
              render: ({ periodEnd }) => (
                <Cell>{displayUnixShortDate(periodEnd)}</Cell>
              ),
            },
            {
              key: 'invoiceId',
              headerText: 'Invoice #',
            },
            {
              key: 'productName',
              headerText: 'Plan',
            },
            {
              key: 'usageMau',
              headerText: 'Active Users',
            },
            {
              key: 'amountDue',
              headerText: 'Invoice Total',
              render: ({ amountDue }) => <Cell>${amountDue}</Cell>,
            },
            {
              key: 'status',
              headerText: 'Status',
            },
            {
              key: 'invoicePdf',
              headerText: 'Action',
              render: ({ invoicePdf }) => renderActionCell(invoicePdf),
            },
          ]}
          emptyText="No Invoices Generated Yet"
        />
      </Box>
    </>
  );
};
