import React, { useMemo, useState } from 'react';
import { Box, ButtonPrimary, Text } from 'design';
import {
  AddressElement,
  useElements,
  useStripe,
} from '@stripe/react-stripe-js';
import { useTheme } from 'styled-components';

import ErrorMessage from 'teleport/components/AgentErrorMessage';

import * as stripeJs from '@stripe/stripe-js';

import { AddressProps } from 'e-teleport/Billing/types';
import { NetworkState } from 'e-teleport/Banner/UsageBasedUpgrade/types';
import useTeleport from 'e-teleport/useTeleportE';
import { StripeBillingAddressRequest } from 'e-teleport/services/cloud';

export const Address = ({ address, name, reload }: AddressProps) => {
  const theme = useTheme();
  const stripe = useStripe();
  const elements = useElements();
  const ctx = useTeleport();

  const [valid, setValid] = useState<boolean>(false);
  const [updated, setUpdated] = useState<boolean>(false);
  const [networkState, setNetworkState] = useState<NetworkState>({});

  const initialState = useMemo(() => {
    return {
      name: name,
      address: {
        line1: address?.addressLine1 || '',
        line2: address?.addressLine2 || '',
        city: address?.addressCity || '',
        state: address?.addressState || '',
        postal_code: address?.addressPostalCode || '',
        country: address?.addressCountry || '',
      },
    };
  }, [name, address]);

  const handleUpdate = (event: stripeJs.StripeAddressElementChangeEvent) => {
    if (event.complete) {
      setValid(true);
    } else {
      setValid(false);
    }

    const updatedValues =
      event.value.name !== initialState.name ||
      event.value.address.country !== initialState.address.country ||
      event.value.address.line1 !== initialState.address.line1 ||
      event.value.address.line2 !== initialState.address.line2 ||
      event.value.address.city !== initialState.address.city ||
      event.value.address.state !== initialState.address.state ||
      event.value.address.postal_code !== initialState.address.postal_code;

    setUpdated(updatedValues);
  };

  const handleClick = async (
    e: React.MouseEvent<HTMLButtonElement>
  ): Promise<void> => {
    e.preventDefault();
    setNetworkState({ status: 'loading', error: undefined });

    const addressElement = elements.getElement('address');
    const { complete, value } = await addressElement.getValue();
    if (!complete) {
      setNetworkState({
        status: 'error',
        error: new Error('Please fill out all address fields.'),
      });
      return;
    }

    const accountReq: StripeBillingAddressRequest = {
      name: value.name,
      address: {
        addressCity: value.address.city,
        addressCountry: value.address.country,
        addressLine1: value.address.line1,
        addressLine2: value.address.line2,
        addressPostalCode: value.address.postal_code,
        addressState: value.address.state,
      },
    };

    ctx.cloudService
      .updateAddress(accountReq)
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
      <h2>Invoice Billing Address</h2>
      <Text>
        Optional: Add a billing address if you would like it to appear on your
        invoices:
      </Text>
      <AddressElement
        options={{
          mode: 'billing',
          defaultValues: initialState,
        }}
        onChange={event => handleUpdate(event)}
      />
      {networkState.error != undefined && (
        <Box mt={2}>
          <ErrorMessage message={networkState.error.message} />
        </Box>
      )}
      <ButtonPrimary
        mt="8px"
        width="200px"
        disabled={
          !stripe || !valid || networkState.status == 'loading' || !updated
        }
        onClick={handleClick}
      >
        Save
      </ButtonPrimary>
    </Box>
  );
};
