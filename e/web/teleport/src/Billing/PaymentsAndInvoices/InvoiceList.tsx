import React from 'react';
import { Box, ButtonSecondary, Link } from 'design';
import Table, { Cell } from 'design/DataTable';

import { displayUnixShortDate } from 'shared/services/loc/loc';
import Label, { Danger, Secondary, Warning } from 'design/Label';

import { InvoiceListProps } from 'e-teleport/Billing/types';

export const InvoiceList = ({ invoices, productName }: InvoiceListProps) => {
  // todo (michellescripts) add "view" action;  https://github.com/gravitational/cloud/issues/3536
  const renderActionCell = (invoiceURL: string): React.ReactElement => {
    return (
      <Cell>
        <Link href={invoiceURL} target="__blank" color="text.primary">
          <ButtonSecondary>Download</ButtonSecondary>
        </Link>
      </Cell>
    );
  };

  const renderLabelCell = (text: string): React.ReactElement => {
    const lower = text.toLowerCase();
    let child;

    switch (lower) {
      case 'paid':
        child = <Label kind="success">{lower}</Label>;
        break;
      case 'uncollectible':
        child = <Danger>{lower}</Danger>;
        break;
      case 'open':
        child = <Warning>{lower}</Warning>;
        break;
      case 'draft':
      case 'void':
      default:
        child = <Secondary>{lower}</Secondary>;
        break;
    }

    return <Cell style={{ width: '40px' }}>{child}</Cell>;
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
              render: ({ usageMau }) => <Cell>{usageMau}</Cell>,
            },
            {
              key: 'amountDue',
              headerText: 'Invoice Total',
              render: ({ amountDue }) => <Cell>{getAmountDue(amountDue)}</Cell>,
            },
            {
              key: 'status',
              headerText: 'Status',
              render: ({ status }) => renderLabelCell(status),
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

export const getAmountDue = (amountDue: number): string => {
  return (amountDue / 100).toLocaleString('en-US', {
    style: 'currency',
    currency: 'USD',
  });
};
