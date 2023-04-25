import React, { useEffect, useState } from 'react';

import { Elements } from '@stripe/react-stripe-js';

import { loadStripe } from '@stripe/stripe-js';

import { BillingInformation } from 'e-teleport/services/cloud';
import useTeleport from 'e-teleport/useTeleportE';

type DataSourceStatus = 'loading' | 'success' | 'error';

interface StripeLoaderState {
  dataSourceStatus: DataSourceStatus;
  data: BillingInformation;
  error?: Error;
}

type ChildRenderer = (
  data: BillingInformation,
  reload: () => void
) => React.ReactNode;

interface StripeLoaderProps {
  render: ChildRenderer;
}

const initialState: StripeLoaderState = {
  dataSourceStatus: 'loading',
  data: undefined,
  error: undefined,
};

/**
 * StripeLoader retrieves billing information including the Stripe public key, and renders wrapped in a Stripe provider
 * @param {ChildRenderer} render - renders the child with the billing information and reload props
 */
export const StripeLoader = ({
  render,
}: StripeLoaderProps): React.ReactElement | null => {
  const ctx = useTeleport();
  const [loaderState, setLoaderState] = useState(initialState);

  const loadData = (): void => {
    setLoaderState(initialState);

    ctx.cloudService
      .fetchBillingInformation()
      .then((data: BillingInformation) => {
        setLoaderState({
          dataSourceStatus: 'success',
          data: data,
        });
      })
      .catch((error: Error) => {
        setLoaderState({
          dataSourceStatus: 'error',
          data: undefined,
          error: error,
        });
      });
  };

  useEffect(loadData, [render, ctx]);

  switch (loaderState.dataSourceStatus) {
    case 'loading':
    case 'error':
      // don't display loading or error states to the user
      return null;
    case 'success':
      return (
        <Elements
          key={`stripe-element-${loaderState.data.stripeCustomerId}`}
          stripe={loadStripe(loaderState.data.stripePublicKey)}
        >
          {render(loaderState.data, loadData)}
        </Elements>
      );
  }
};
