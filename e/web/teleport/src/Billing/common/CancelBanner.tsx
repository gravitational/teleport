import { Box, ButtonSecondary, Text } from 'design';
import React from 'react';
import { useTheme } from 'styled-components';

import { displayUnixShortDate } from 'shared/services/loc/loc';

import { CancelBannerProps } from 'e-teleport/Billing/types';

export const CancelBanner = ({
  productName,
  stripeTrialEnd,
  stripeMissingPaymentMethod,
}: CancelBannerProps) => {
  const theme = useTheme();

  const title = stripeMissingPaymentMethod
    ? `${productName} Trial is Activated`
    : `${productName} Plan is Activated`;

  return (
    <Box
      bg={theme.colors.spotBackground[0]}
      borderRadius="12px"
      m="20px 0 0 0"
      p="20px 0 20px 40px"
    >
      <h2>{title}</h2>
      {stripeMissingPaymentMethod && (
        <Text color={theme.colors.text.secondary}>
          Your trial will expire on {displayUnixShortDate(stripeTrialEnd)}. To
          maintain access to your Teleport cluster, upgrade to the Teleport Team
          Plan.
        </Text>
      )}
      {!stripeMissingPaymentMethod && (
        //   todo (michellescripts) tie into new cancel flow (timothyb89)
        <ButtonSecondary mt="12px">Cancel Plan</ButtonSecondary>
      )}
    </Box>
  );
};
