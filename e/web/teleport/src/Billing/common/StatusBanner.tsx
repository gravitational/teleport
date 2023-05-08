import { Box, ButtonSecondary, Text } from 'design';
import React from 'react';
import { useTheme } from 'styled-components';

import { displayUnixShortDate } from 'shared/services/loc/loc';

import { StatusBannerProps } from 'e-teleport/Billing/types';
import { StripeSubscriptionStatus } from 'e-teleport/Billing/StripeLoader/types';

export const StatusBanner = ({
  productName,
  stripeTrialEnd,
  stripeSubscriptionStatus,
  stripeMissingPaymentMethod,
}: StatusBannerProps) => {
  const theme = useTheme();

  if (stripeSubscriptionStatus === StripeSubscriptionStatus.CANCELED) {
    return null;
  }

  const title = (): string => {
    switch (stripeSubscriptionStatus) {
      case StripeSubscriptionStatus.TRIALING:
        if (!stripeMissingPaymentMethod) {
          return `${productName} Plan is Activated`;
        }
        return `${productName} Trial is Activated`;
      case StripeSubscriptionStatus.ACTIVE:
        return `${productName} Plan is Activated`;
      case StripeSubscriptionStatus.PAST_DUE:
      case StripeSubscriptionStatus.UNPAID:
        return `There is an issue with your ${productName} Plan`;
      case StripeSubscriptionStatus.INCOMPLETE:
      case StripeSubscriptionStatus.INCOMPLETE_EXPIRED:
        return `You're on the ${productName} Plan`;
    }
  };

  const description = (): string => {
    switch (stripeSubscriptionStatus) {
      case StripeSubscriptionStatus.TRIALING:
        if (!stripeMissingPaymentMethod) {
          return `Your trial will expire on ${displayUnixShortDate(
            stripeTrialEnd
          )}.`;
        }
        return `Your trial will expire on ${displayUnixShortDate(
          stripeTrialEnd
        )}. To maintain access to your Teleport cluster, upgrade to the Teleport ${productName} Plan.`;
      case StripeSubscriptionStatus.PAST_DUE:
      case StripeSubscriptionStatus.UNPAID:
        return 'Please check your payment details.';
      case StripeSubscriptionStatus.ACTIVE:
      case StripeSubscriptionStatus.INCOMPLETE:
      case StripeSubscriptionStatus.INCOMPLETE_EXPIRED:
        return null;
    }
  };

  return (
    <Box
      bg={theme.colors.spotBackground[0]}
      borderRadius="12px"
      m="20px 0 0 0"
      p="20px 0 20px 40px"
    >
      <h2>{title()}</h2>
      <Text color={theme.colors.text.secondary}>{description()}</Text>
      {(stripeSubscriptionStatus === StripeSubscriptionStatus.ACTIVE ||
        (stripeSubscriptionStatus === StripeSubscriptionStatus.TRIALING &&
          !stripeMissingPaymentMethod)) && (
        //   todo (michellescripts) tie into new cancel flow (timothyb89)
        <ButtonSecondary mt="12px">Cancel Plan</ButtonSecondary>
      )}
    </Box>
  );
};
