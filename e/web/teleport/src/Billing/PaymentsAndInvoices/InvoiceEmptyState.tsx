import React from 'react';
import { Box } from 'design';
import { useTheme } from 'styled-components';

import { EmptySVG } from 'e-teleport/Billing/PaymentsAndInvoices/EmptySVG';

export const InvoiceEmptyState = (): React.ReactElement => {
  const theme = useTheme();
  return (
    <Box
      bg={theme.colors.spotBackground[0]}
      borderRadius="12px"
      m="20px 0"
      p="20px 0 20px 40px"
      textAlign="center"
    >
      <EmptySVG />
      <h3> No Invoices Generated Yet </h3>
    </Box>
  );
};
