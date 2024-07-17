import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/Dialog';

import { ButtonPrimary, ButtonSecondary, H2, Input, Text } from 'design';
import React, { useState } from 'react';
import { useStripe } from '@stripe/react-stripe-js';

import { Danger } from 'design/Alert';

import { useTheme } from 'styled-components';

import { displayUnixShortDate } from 'shared/services/loc/loc';

import { NetworkState } from 'e-teleport/Banner/UsageBasedUpgrade/types';
import { CancelDialogProps } from 'e-teleport/Billing/types';
import useTeleport from 'e-teleport/useTeleportE';

const CONFIRM_TEXT = 'close my account';

export const CancelAccountDialog = ({
  open,
  setOpen,
  productName,
  stripeCurrentPeriodEnd,
  tenant,
}: CancelDialogProps) => {
  const theme = useTheme();
  const ctx = useTeleport();
  const stripe = useStripe();
  const [networkState, setNetworkState] = useState<NetworkState>({});
  const [confirmed, setConfirmed] = useState<string>('');
  const [subdomain, setSubdomain] = useState<string>('');

  const hasErrorSubdomainInput =
    subdomain && subdomain.toLowerCase() !== tenant;
  const hasErrorConfirmInput =
    confirmed && confirmed.toLowerCase() !== CONFIRM_TEXT;

  const dialogText: React.ReactNode =
    stripeCurrentPeriodEnd != 0 ? (
      <Text>
        You are about to cancel your Teleport {productName} Plan. If you
        continue, at the end of your billing cycle on{' '}
        <b>{displayUnixShortDate(stripeCurrentPeriodEnd)}</b>:
      </Text>
    ) : (
      <Text>
        You are about to cancel your Teleport {productName} Plan. If you
        continue, at the end of your billing cycle:
      </Text>
    );

  const handleDelete = (): void => {
    setNetworkState({ status: 'loading' });

    ctx.cloudService
      .cancelSubscription()
      .then(() => {
        window.location.reload();
      })
      .catch(error => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  return (
    <Dialog open={open}>
      <DialogHeader>
        <H2 color={theme.colors.text.main}>Close Teleport Account</H2>
      </DialogHeader>
      <DialogContent maxWidth="636px">
        {dialogText}
        <ul>
          <li>Access to Teleport will be cut off.</li>
          <li>Teleport will issue a final invoice.</li>
          <li>Your entire account will be deleted 7–14 days later.</li>
        </ul>
        <Text mb={4} color={theme.colors.text.slightlyMuted} bold>
          Once deleted, your Teleport account cannot be recovered.
        </Text>
        <label
          htmlFor="subdomain"
          style={
            hasErrorSubdomainInput ? { color: theme.colors.error.main } : {}
          }
          data-testid="label-subdomain"
        >
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
          hasError={hasErrorSubdomainInput}
          data-testid="input-subdomain"
        />
        <label
          htmlFor="confirmed"
          style={hasErrorConfirmInput ? { color: theme.colors.error.main } : {}}
          data-testid="label-confirmed"
        >
          Please confirm your choice by typing <b>{CONFIRM_TEXT}</b> below:
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
          hasError={hasErrorConfirmInput}
          data-testid="input-confirmed"
        />
      </DialogContent>
      {networkState.error != undefined && (
        <Danger>{networkState.error.message}</Danger>
      )}
      <DialogFooter>
        <ButtonPrimary
          mr="3"
          disabled={
            !stripe ||
            networkState.status == 'loading' ||
            subdomain.toLowerCase() !== tenant ||
            confirmed.toLowerCase() !== CONFIRM_TEXT
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
