import React, { useState } from 'react';
import { ButtonPrimary, Flex, Text } from 'design';

import { displayShortDate } from 'shared/services/loc/loc';

import { add, differenceInDays, fromUnixTime } from 'date-fns';

import { PaymentAddDialog } from 'e-teleport/Billing/Payment/PaymentAddDialog';

import { UsageBasedUpgradeProps } from 'e-teleport/Banner/UsageBasedUpgrade/types';

// todo (michellescripts) we need to listen for payment methods being added, otherwise the banner won't reload; part of https://github.com/gravitational/cloud/issues/3536
export const UsageBasedUpgrade = ({
  billingInfo: {
    productName,
    stripeMissingPaymentMethod,
    stripeTrial,
    stripeTrialEnd,
    usageBasedBilling,
  },
  reload,
}: UsageBasedUpgradeProps) => {
  const trialEndDate = fromUnixTime(stripeTrialEnd);
  const dayAfterTrial = displayShortDate(add(trialEndDate, { days: 1 }));
  // remaining days should include the final day in addition to the days between
  const remainingDays = differenceInDays(trialEndDate, new Date()) + 1;
  const remainingDaysText =
    remainingDays === 1
      ? 'Your trial expires today'
      : `Your trial expires in ${remainingDays} days`;

  const [open, setOpen] = useState<boolean>(false);

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
          <ButtonPrimary onClick={() => setOpen(true)}>Upgrade</ButtonPrimary>
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
