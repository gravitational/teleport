import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import { ButtonPrimary, ButtonSecondary, Text } from 'design';
import React, { useState } from 'react';

import * as setupIntents from '@stripe/stripe-js/types/stripe-js/setup-intents';
import { CardElement, useElements, useStripe } from '@stripe/react-stripe-js';

import { CheckboxInput, CheckboxWrapper } from 'design/Checkbox';

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
  makeDefault = false,
  showDefaultOption = false,
}: PaymentAddDialogProps) => {
  const ctx = useTeleport();
  const stripe = useStripe();
  const elements = useElements();

  const [valid, setValid] = useState<boolean>(false);
  const [networkState, setNetworkState] = useState<NetworkState>({});
  const [primary, setPrimary] = useState<boolean>(makeDefault || false);

  const handleClick = (e: React.MouseEvent<HTMLButtonElement>): void => {
    e.preventDefault();
    setNetworkState({ status: 'loading', error: undefined });

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

  // confirmCard confirms a setup intent and adds a payment method https://stripe.com/docs/js/setup_intents/confirm_card_setup
  const confirmCard = (clientSecret: string): void => {
    const data: setupIntents.ConfirmCardSetupData = {
      payment_method: {
        card: elements.getElement(CardElement),
      },
    };

    stripe
      .confirmCardSetup(clientSecret, data)
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
        setOpen(false);
        setNetworkState({ status: undefined });
        reload();
      })
      .catch(error => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  return (
    <Dialog open={open}>
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
      </DialogHeader>
      <DialogContent width="400px">
        {description && <Text>{description}</Text>}
        <CreditCard setValid={setValid} />
        {showDefaultOption && (
          <CheckboxWrapper
            as="label"
            htmlFor="setDefault"
            style={{ border: 'none' }}
          >
            <CheckboxInput
              type="checkbox"
              name="Make Default Payment"
              id="setDefault"
              data-testid="set-default"
              onChange={e => {
                setPrimary(e.target.checked);
              }}
              checked={primary}
            />
            Make Default Payment
          </CheckboxWrapper>
        )}
      </DialogContent>
      {networkState.error != undefined && (
        <Text>{networkState.error.message}</Text>
      )}
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
  );
};
