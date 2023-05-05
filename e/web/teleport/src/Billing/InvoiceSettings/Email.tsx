import React, { useState } from 'react';
import { ButtonPrimary, Input, Text } from 'design';

import { EmailProps } from 'e-teleport/Billing/types';
import { NetworkState } from 'e-teleport/Banner/UsageBasedUpgrade/types';
import useTeleport from 'e-teleport/useTeleportE';
import { isValidEmail } from 'e-teleport/validations/email';

export const Email = ({ email }: EmailProps) => {
  const ctx = useTeleport();
  const [field, setField] = useState<string>(email ? email : '');
  const [networkState, setNetworkState] = useState<NetworkState>({});

  const handleClick = (e: React.MouseEvent<HTMLButtonElement>): void => {
    e.preventDefault();
    setNetworkState({ status: 'loading', error: undefined });

    if (!isValidEmail(field)) {
      setNetworkState({
        status: 'error',
        error: new Error('Email format is invalid'),
      });
      return;
    }

    ctx.cloudService
      .updateEmail({ email: field })
      .then(() => {
        setNetworkState({ status: undefined });
      })
      .catch(error => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  return (
    <>
      <h2>Invoice Email Recipient</h2>
      <Text>
        Optional: Invoices will be sent to the email address of the cluster's
        creator by default. Add an address here if you want invoices to go to a
        different address, instead:
      </Text>
      <Input
        aria-label="email"
        mt={3}
        type="email"
        label="Email Address"
        value={field}
        placeholder="invoices@mycompany.com"
        onChange={e => {
          setField(e.target.value);
        }}
      />
      {networkState.error != undefined && (
        <Text>{networkState.error.message}</Text>
      )}
      <ButtonPrimary
        mt="8px"
        width="200px"
        type="submit"
        onClick={handleClick}
        disabled={networkState.status == 'loading'}
      >
        Save
      </ButtonPrimary>
    </>
  );
};
