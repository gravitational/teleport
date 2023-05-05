import React, { useState } from 'react';
import { ButtonPrimary, Input, Text } from 'design';

import { PurchaseOrderProps } from 'e-teleport/Billing/types';
import { NetworkState } from 'e-teleport/Banner/UsageBasedUpgrade/types';
import useTeleport from 'e-teleport/useTeleportE';
import { isValidPurchaseOrderPrefix } from 'e-teleport/validations/purchaseOrderPrefix';

export const PurchaseOrder = ({ po }: PurchaseOrderProps) => {
  const ctx = useTeleport();
  const [field, setField] = useState<string>(po ? po : '');
  const [networkState, setNetworkState] = useState<NetworkState>({});

  const handleClick = (e: React.MouseEvent<HTMLButtonElement>): void => {
    e.preventDefault();
    setNetworkState({ status: 'loading', error: undefined });

    if (!isValidPurchaseOrderPrefix(field)) {
      setNetworkState({
        status: 'error',
        error: new Error(
          'Purchase order format is invalid. Prefix must be 3–12 letters or numbers.'
        ),
      });
      return;
    }

    ctx.cloudService
      .updatePurchaseOrderPrefix({ po: field.toUpperCase() })
      .then(() => {
        setNetworkState({ status: undefined });
      })
      .catch(error => {
        setNetworkState({ status: 'error', error: error });
      });
  };

  return (
    <>
      <h2>Invoice Purchase Order</h2>
      <Text>
        Optional: Add a purchase order below if you would like one to show up on
        your invoices.
      </Text>
      <Input
        aria-label="purchase order"
        mt={3}
        type="text"
        label="purchase order"
        value={field}
        placeholder="PO Number for Invoices"
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
