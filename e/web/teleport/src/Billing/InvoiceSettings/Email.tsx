import React, { useState } from 'react';
import { Box, ButtonPrimary, Input, Text } from 'design';

import ErrorMessage from 'teleport/components/AgentErrorMessage';

import { useTheme } from 'styled-components';

import { EmailProps } from 'e-teleport/Billing/types';
import { NetworkState } from 'e-teleport/Banner/UsageBasedUpgrade/types';
import useTeleport from 'e-teleport/useTeleportE';
import { isValidEmail } from 'e-teleport/validations/email';

export const Email = ({ email, reload }: EmailProps) => {
  const theme = useTheme();
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
        reload();
      })
      .catch(error => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  return (
    <Box
      bg={theme.colors.levels.surface}
      borderRadius="8px"
      m="20px 0 0 0"
      p="20px 24px 20px 40px"
    >
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
        value={field}
        placeholder="invoices@mycompany.com"
        onChange={e => {
          setField(e.target.value);
        }}
      />
      {networkState.error != undefined && (
        <Box mt={2}>
          <ErrorMessage message={networkState.error.message} />
        </Box>
      )}
      <ButtonPrimary
        mt="8px"
        width="200px"
        type="submit"
        onClick={handleClick}
        disabled={
          networkState.status == 'loading' || field === email || field === ''
        }
      >
        Save
      </ButtonPrimary>
    </Box>
  );
};
