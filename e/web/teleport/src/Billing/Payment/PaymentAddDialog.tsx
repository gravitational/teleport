import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/Dialog';

import { ButtonPrimary, ButtonSecondary, Text } from 'design';
import React, { useState } from 'react';
import { useElements, useStripe } from '@stripe/react-stripe-js';
import { useTheme } from 'styled-components';

import { Danger } from 'design/Alert';

import useStickyClusterId from 'teleport/useStickyClusterId';

import { FieldCheckbox } from 'shared/components/FieldCheckbox';

import { SetupIntent } from 'e-teleport/services/cloud';
import { CreditCard } from 'e-teleport/Banner/UsageBasedUpgrade/CreditCard';
import { NetworkState } from 'e-teleport/Banner/UsageBasedUpgrade/types';
import useTeleport from 'e-teleport/useTeleportE';
import { PaymentAddDialogProps } from 'e-teleport/Billing/types';

export const PaymentAddDialog = ({
  open,
  setOpen,
  reload,
  title,
  description,
  stripeMissingPaymentMethod,
  makeDefault = false,
  showDefaultOption = false,
}: PaymentAddDialogProps) => {
  const theme = useTheme();
  const ctx = useTeleport();
  const stripe = useStripe();
  const elements = useElements();
  const { clusterId } = useStickyClusterId();

  const [valid, setValid] = useState<boolean>(false);
  const [networkState, setNetworkState] = useState<NetworkState>({});
  const [primary, setPrimary] = useState<boolean>(makeDefault || false);

  const handleClick = async (
    e: React.MouseEvent<HTMLButtonElement>
  ): Promise<void> => {
    e.preventDefault();
    setNetworkState({ status: 'loading', error: undefined });

    // validate the state of the payment element
    const { error: submitError } = await elements.submit();
    if (submitError) {
      setNetworkState({ status: undefined, error: submitError });
    }

    // begin the add payment flow
    createSetupIntent();
  };

  // createSetupIntent is used to set up recurring payments with a final amount determined later https://stripe.com/docs/api/setup_intents/create
  const createSetupIntent = (): void => {
    ctx.cloudService
      .createSetupIntent()
      .then((data: SetupIntent) => {
        confirmCard(data.clientSecret);
      })
      .catch((error: Error) => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  // confirmCard confirms a setup intent and adds a payment method https://stripe.com/docs/js/setup_intents/confirm_setup
  const confirmCard = (clientSecret: string): void => {
    stripe
      .confirmSetup({
        elements,
        clientSecret,
        // we only want redirect for redirect-based payments
        redirect: 'if_required',
      })
      .then(result => {
        if (result.setupIntent != undefined) {
          // Our request does not expand the payment_method field;
          // therefore the function will return a string rather than a
          // PaymentMethod and this cast is safe
          addCard(result.setupIntent.payment_method as string);
        }
        if (result.error != undefined) {
          setNetworkState({ status: 'error', error: result.error });
        }
      })
      .catch(error => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  // addCard is a cloud function that attaches the payment method to the customer https://stripe.com/docs/api/payment_methods/attach
  // and sets the card as primary on the customer invoice https://stripe.com/docs/api/customers/update#update_customer-invoice_settings-primary_payment_method
  const addCard = (cardID: string): void => {
    ctx.cloudService
      .addCard({ cardId: cardID, isDefault: primary })
      .then(() => {
        // TODO(mcbattirola): capture user event.
        setOpen(false);
        setNetworkState({ status: undefined });
        if (stripeMissingPaymentMethod) {
          // rerender the page to trigger a refresh for both the banner CTA and the page CTA
          window.location.reload();
        } else {
          // rerender only the parent component
          reload();
        }
      })
      .catch(error => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  return (
    <Dialog open={open}>
      <DialogHeader>
        <Text typography="h3" color={theme.colors.text.main}>
          {title}
        </Text>
      </DialogHeader>
      <DialogContent width="400px">
        <Text mb={3} color={theme.colors.text.slightlyMuted}>
          {description
            ? description
            : `Add a payment method for your cluster, ${clusterId}`}
        </Text>
        <CreditCard setValid={setValid} />
        {showDefaultOption && (
          <FieldCheckbox
            label="Make Default Payment"
            name="Make Default Payment"
            data-testid="set-default"
            onChange={e => {
              setPrimary(e.target.checked);
            }}
            checked={primary}
          />
        )}
      </DialogContent>
      {networkState.error != undefined && (
        <Danger>{networkState.error.message}</Danger>
      )}
      <DialogFooter>
        <ButtonPrimary
          mr={4}
          width="45%"
          disabled={!stripe || !valid || networkState.status == 'loading'}
          onClick={handleClick}
        >
          Save And Close
        </ButtonPrimary>
        <ButtonSecondary
          width="45%"
          disabled={networkState.status == 'loading'}
          onClick={() => setOpen(false)}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
};
