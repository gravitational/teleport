import React, { useState } from 'react';
import { Box, ButtonPrimary, Flex, Text } from 'design';

import { displayShortDate } from 'shared/services/loc/loc';

import { add, differenceInDays, fromUnixTime } from 'date-fns';

import { CtaEvent, userEventService } from 'teleport/services/userEvent';

import { Warning } from 'design/Icon';

import { useTheme } from 'styled-components';

import Link from 'design/Link';

import { PaymentAddDialog } from 'e-teleport/Billing/Payment/PaymentAddDialog';

import { UsageBasedUpgradeProps } from 'e-teleport/Banner/UsageBasedUpgrade/types';
import { StripeSubscriptionStatus } from 'e-teleport/Billing/StripeLoader/types';

export const UsageBasedUpgrade = ({
  billingInfo: {
    productName,
    stripeMissingPaymentMethod,
    stripeTrial,
    stripeTrialEnd,
    usageBasedBilling,
    stripeSubscriptionStatus,
  },
  reload,
}: UsageBasedUpgradeProps) => {
  const theme = useTheme();

  function handleClick() {
    userEventService.captureCtaEvent(CtaEvent.CTA_UPGRADE_BANNER);
    setOpen(true);
  }

  const trialEndDate = fromUnixTime(stripeTrialEnd);
  const dayAfterTrial = displayShortDate(add(trialEndDate, { days: 1 }));
  // remaining days should include the final day in addition to the days between
  const remainingDays = differenceInDays(trialEndDate, new Date()) + 1;
  const remainingDaysText =
    remainingDays === 1
      ? 'Your trial expires today'
      : `Your trial expires in ${remainingDays} days`;

  const [open, setOpen] = useState<boolean>(false);

  if (
    usageBasedBilling === true &&
    stripeSubscriptionStatus === StripeSubscriptionStatus.CANCELED
  ) {
    return (
      <Box
        bg={theme.colors.error.main}
        color={theme.colors.text.primaryInverse}
        p={1}
        pl={2}
      >
        <Flex alignItems="center">
          <Warning
            mr={3}
            fontSize="3"
            role="icon"
            color={theme.colors.text.primaryInverse}
          />
          <Text>
            Your account has been canceled. Your account will be deleted once
            the grace period is over. To cancel this process, please&nbsp;
            <Link
              color={theme.colors.text.primaryInverse}
              href="https://goteleport.com/support/"
              target="_blank"
            >
              contact us.
            </Link>
          </Text>
        </Flex>
      </Box>
    );
  }

  if (usageBasedBilling === false || stripeTrial === false) {
    return null;
  }

  return (
    <Flex
      height="38px"
      bg="levels.surfaceSecondary"
      justifyContent="center"
      data-testid="upgrade-banner"
    >
      {stripeMissingPaymentMethod ? (
        <Flex alignItems="center" gap="20px">
          <Text mr={1} data-testid="message">
            {remainingDaysText}. To maintain access Upgrade{' '}
            <b>{productName} Trial</b> to <b>{productName} Plan</b>.
          </Text>
          <ButtonPrimary onClick={handleClick}>Upgrade</ButtonPrimary>
        </Flex>
      ) : (
        <Text mr={1} data-testid="message">
          {remainingDaysText}. Your first monthly billing cycle will start on{' '}
          {dayAfterTrial}.
        </Text>
      )}
      {open && (
        <PaymentAddDialog
          open={open}
          reload={reload}
          setOpen={setOpen}
          makeDefault={stripeMissingPaymentMethod}
          stripeMissingPaymentMethod={stripeMissingPaymentMethod}
          title={`Upgrade to ${productName}`}
          description={`Add a payment method to automatically upgrade your account to the 
          ${productName} plan when your trial ends on ${displayShortDate(
            trialEndDate
          )}`}
        />
      )}
    </Flex>
  );
};
