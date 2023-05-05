import React, { useState } from 'react';
import { ButtonPrimary, Text } from 'design';
import {
  AddressElement,
  useElements,
  useStripe,
} from '@stripe/react-stripe-js';

import { AddressProps } from 'e-teleport/Billing/types';
import { NetworkState } from 'e-teleport/Banner/UsageBasedUpgrade/types';
import useTeleport from 'e-teleport/useTeleportE';
import { StripeBillingAddressRequest } from 'e-teleport/services/cloud';

export const Address = ({ address, name }: AddressProps) => {
  const stripe = useStripe();
  const elements = useElements();
  const ctx = useTeleport();

  const [valid, setValid] = useState<boolean>(false);
  const [networkState, setNetworkState] = useState<NetworkState>({});

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
        setNetworkState({ status: undefined });
      })
      .catch(error => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  return (
    <>
      <h2>Invoice Billing Address</h2>
      <Text>
        Optional: Add a billing address if you would like it to appear on your
        invoices:
      </Text>
      <AddressElement
        options={{
          mode: 'billing',
          defaultValues: {
            name: name,
            address: {
              line1: address?.addressLine1 || '',
              line2: address?.addressLine2 || '',
              city: address?.addressCity || '',
              state: address?.addressState || '',
              postal_code: address?.addressPostalCode || '',
              country: address?.addressCountry || '',
            },
          },
        }}
        onChange={event => {
          if (event.complete) {
            setValid(true);
          } else {
            setValid(false);
          }
        }}
      />
      {networkState.error != undefined && (
        <Text>{networkState.error.message}</Text>
      )}
      <ButtonPrimary
        mt="8px"
        width="200px"
        disabled={!stripe || !valid || networkState.status == 'loading'}
        onClick={handleClick}
      >
        Save
      </ButtonPrimary>
    </>
  );
};
