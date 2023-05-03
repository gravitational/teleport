import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import * as Icons from 'design/Icon';

import { ButtonPrimary, ButtonSecondary, Text } from 'design';
import React, { useState } from 'react';
import { useStripe } from '@stripe/react-stripe-js';

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
        <DialogTitle>
          Are you sure you want to delete this payment method?
        </DialogTitle>
      </DialogHeader>
      <DialogContent width="400px">
        <Icons.Info />
        <p>
          You are about to delete {brand} {last4} from your account. Once
          removed, it will be unavailable for use.
        </p>
      </DialogContent>
      {networkState.error != undefined && (
        <Text>{networkState.error.message}</Text>
      )}
      <DialogFooter>
        <ButtonPrimary
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
