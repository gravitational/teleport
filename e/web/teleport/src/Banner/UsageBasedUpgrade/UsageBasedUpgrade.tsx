import React, { useState } from 'react';
import { ButtonPrimary, ButtonSecondary, Flex, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { CardElement, useElements, useStripe } from '@stripe/react-stripe-js';

import * as setupIntents from '@stripe/stripe-js/types/stripe-js/setup-intents';

import { displayShortDate } from 'shared/services/loc/loc';
import { add, differenceInDays, fromUnixTime } from 'date-fns';

import { CreditCard } from 'e-teleport/Banner/UsageBasedUpgrade/CreditCard';
import { SetupIntent } from 'e-teleport/services/cloud';
import useTeleport from 'e-teleport/useTeleportE';
import {
  NetworkState,
  UsageBasedUpgradeProps,
} from 'e-teleport/Banner/UsageBasedUpgrade/types';

export const UsageBasedUpgrade = ({
  billingInfo: {
    productName,
    usageBasedBilling,
    stripeTrial,
    stripeTrialEnd,
    stripeMissingPaymentMethod,
  },
  reload,
}: UsageBasedUpgradeProps) => {
  const ctx = useTeleport();
  const stripe = useStripe();
  const elements = useElements();

  const trialEndDate = fromUnixTime(stripeTrialEnd);
  const dayAfterTrial = displayShortDate(add(trialEndDate, { days: 1 }));
  // remaining days should include the final day in addition to the days between
  const remainingDays = differenceInDays(trialEndDate, new Date()) + 1;
  const remainingDaysText =
    remainingDays === 1
      ? 'Your trial expires today'
      : `Your trial expires in ${remainingDays} days`;

  const [open, setOpen] = useState<boolean>(false);
  const [valid, setValid] = useState<boolean>(false);
  const [networkState, setNetworkState] = useState<NetworkState>({});

  const handleClick = (e: React.MouseEvent<HTMLButtonElement>) => {
    e.preventDefault();
    setNetworkState({ status: 'loading', error: undefined });

    // https://stripe.com/docs/api/setup_intents/create
    ctx.cloudService
      .createSetupIntent()
      .then((data: SetupIntent) => {
        addCard(data.clientSecret);
      })
      .catch((error: Error) => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  const addCard = (clientSecret: string) => {
    const data: setupIntents.ConfirmCardSetupData = {
      payment_method: { card: elements.getElement(CardElement) },
    };

    // https://stripe.com/docs/js/setup_intents/confirm_card_setup
    stripe
      .confirmCardSetup(clientSecret, data)
      .then(result => {
        if (result.setupIntent != undefined) {
          setOpen(false);
          setNetworkState({ status: undefined });
          reload();
        }
        if (result.error != undefined) {
          setNetworkState({ status: 'error', error: result.error });
        }
      })
      .catch(error => {
        setNetworkState({ status: 'error', error: error });
      });
  };

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
        <Dialog open={open}>
          <DialogHeader>
            <DialogTitle>Upgrade to {productName}</DialogTitle>
          </DialogHeader>
          <DialogContent width="400px">
            <p>
              Add a payment method to automatically upgrade your account to the{' '}
              {productName} plan when your trial ends on{' '}
              {displayShortDate(trialEndDate)}
            </p>
            <CreditCard setValid={setValid} />
          </DialogContent>
          {networkState.error != undefined && <Text>{networkState.error}</Text>}
          <DialogFooter>
            <ButtonPrimary
              disabled={!stripe || !valid || networkState.status == 'loading'}
              onClick={handleClick}
            >
              Save And Close
            </ButtonPrimary>
            <ButtonSecondary
              disabled={networkState.status == 'loading'}
              onClick={() => setOpen(false)}
            >
              Cancel
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      )}
    </Flex>
  );
};
