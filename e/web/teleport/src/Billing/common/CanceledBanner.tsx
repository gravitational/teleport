import { Box } from 'design';
import React from 'react';
import { useTheme } from 'styled-components';

import { CancelText } from 'e-teleport/Billing/common/CancelText';
import { CancelAtProps } from 'e-teleport/Billing/types';

export const CanceledBanner = ({
  stripeSubscriptionCancelAt,
}: CancelAtProps) => {
  const theme = useTheme();

  return (
    <Box
      bg={theme.colors.error.main}
      color={theme.colors.text.primaryInverse}
      m="0 -40px"
      p="20px 0 20px 40px"
    >
      <h2>Your account is canceled</h2>
      <i>
        <CancelText stripeSubscriptionCancelAt={stripeSubscriptionCancelAt} />
      </i>
    </Box>
  );
};
