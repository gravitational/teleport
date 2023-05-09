import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import { ButtonPrimary, ButtonSecondary, Input, Text } from 'design';
import React, { useState } from 'react';
import { useStripe } from '@stripe/react-stripe-js';

import { NetworkState } from 'e-teleport/Banner/UsageBasedUpgrade/types';
import { CancelDialogProps } from 'e-teleport/Billing/types';
import useTeleport from 'e-teleport/useTeleportE';

export const CancelAccountDialog = ({
  open,
  setOpen,
  tenant,
}: CancelDialogProps) => {
  const ctx = useTeleport();
  const stripe = useStripe();
  const [networkState, setNetworkState] = useState<NetworkState>({});
  const [confirmed, setConfirmed] = useState<string>('');
  const [subdomain, setSubdomain] = useState<string>('');

  const handleDelete = (): void => {
    setNetworkState({ status: 'loading' });

    ctx.cloudService
      .cancelSubscription()
      .then(() => {
        setOpen(false);
        setNetworkState({ status: undefined });
      })
      .catch(error => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  // todo (michellescripts) update copy to align with billing cycle as part of https://github.com/gravitational/cloud/issues/3536
  return (
    <Dialog open={open}>
      <DialogHeader>
        <DialogTitle>Close Teleport Account</DialogTitle>
      </DialogHeader>
      <DialogContent width="400px">
        <Text>
          You are about to cancel your Teleport Team Plan. If you cancel your
          Teleport plan:
        </Text>
        <ul>
          <li>
            Teleport will issue a final pro-rated charge for any usage during
            the current billing period.
          </li>
          <li>Access to Teleport resources will be cut off.</li>
          <li>Your entire account will be deleted in 7-14 days.</li>
        </ul>
        <Text>Once deleted, your Teleport account cannot be recovered.</Text>
        <label htmlFor="subdomain">
          Please confirm your cluster's subdomain, <b>{tenant}</b> below:
        </label>
        <Input
          aria-label="subdomain"
          aria-required={true}
          mb={3}
          type="subdomain"
          value={subdomain}
          placeholder={tenant}
          onChange={e => {
            setSubdomain(e.target.value);
          }}
        />
        <label htmlFor="confirmed">
          Please confirm your choice by typing <b>close my account</b> below:
        </label>
        <Input
          aria-label="confirmed"
          aria-required={true}
          mb={3}
          type="confirmed"
          value={confirmed}
          placeholder="close my account"
          onChange={e => {
            setConfirmed(e.target.value);
          }}
        />
      </DialogContent>
      {networkState.error != undefined && (
        <Text>{networkState.error.message}</Text>
      )}
      <DialogFooter>
        <ButtonPrimary
          mr="3"
          disabled={
            !stripe ||
            networkState.status == 'loading' ||
            subdomain.toLowerCase() !== tenant ||
            confirmed.toLowerCase() !== 'close my account'
          }
          onClick={handleDelete}
        >
          Cancel Plan and Close Account
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
