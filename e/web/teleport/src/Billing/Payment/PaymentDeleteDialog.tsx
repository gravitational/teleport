import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/Dialog';

import { ButtonPrimary, ButtonSecondary } from 'design';
import React, { useState } from 'react';
import { useStripe } from '@stripe/react-stripe-js';

import { Danger } from 'design/Alert';

import { H2 } from 'design/Text';

import { NetworkState } from 'e-teleport/Banner/UsageBasedUpgrade/types';
import { ExistingPaymentProps } from 'e-teleport/Billing/types';
import useTeleport from 'e-teleport/useTeleportE';

export const PaymentDeleteDialog = ({
  open,
  setOpen,
  card: { brand, last4, id },
  reload,
}: ExistingPaymentProps) => {
  const ctx = useTeleport();
  const stripe = useStripe();
  const [networkState, setNetworkState] = useState<NetworkState>({});

  const handleDelete = (): void => {
    setNetworkState({ status: 'loading' });

    ctx.cloudService
      .removeCard({ cardId: id })
      .then(() => {
        setOpen(false);
        reload();
      })
      .catch(error => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  return (
    <Dialog open={open}>
      <DialogHeader>
        <H2>Are you sure you want to delete this payment method?</H2>
      </DialogHeader>
      <DialogContent>
        <p>
          You are about to delete{' '}
          <b>
            {brand.charAt(0).toUpperCase() + brand.slice(1)} *{last4}
          </b>{' '}
          from your account. Once removed, it will be unavailable for use.
        </p>
      </DialogContent>
      {networkState.error != undefined && (
        <Danger>{networkState.error.message}</Danger>
      )}
      <DialogFooter>
        <ButtonPrimary
          mr="3"
          disabled={!stripe || networkState.status == 'loading'}
          onClick={handleDelete}
        >
          Delete Card
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
