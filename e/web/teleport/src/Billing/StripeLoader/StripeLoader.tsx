import React, { useEffect, useState } from 'react';

// Note: using the pure version of stripe-js to avoid loading it if there's no
// need (and our CSP prevents it). See
// https://github.com/gravitational/teleport.e/issues/2986.
import { Elements } from '@stripe/react-stripe-js';
import { loadStripe } from '@stripe/stripe-js/pure';

import { useTheme } from 'styled-components';

import { Box } from 'design';

import CloudService, {
  BillingInformation,
  BillingSummaryInformation,
  InvoiceSettingsInformation,
  PaymentsInvoicesInformation,
} from 'e-teleport/services/cloud';
import useTeleport from 'e-teleport/useTeleportE';

import { getAppearance } from 'e-teleport/Billing/StripeLoader/appearance';
type DataSourceStatus = 'loading' | 'success' | 'error';

interface StripeLoaderState {
  dataSourceStatus: DataSourceStatus;
  data: Response;
  error?: Error;
}

type Response =
  | BillingInformation
  | BillingSummaryInformation
  | PaymentsInvoicesInformation
  | InvoiceSettingsInformation;

type ChildRenderer = (data: Response, reload: () => void) => React.ReactNode;

type DataSource = (CloudService: CloudService) => Promise<Response>;

interface StripeLoaderProps {
  dataSource: DataSource;
  render: ChildRenderer;
  errorRender?: (error: Error) => React.ReactNode;
  loadingRender?: () => React.ReactNode;
}

const initialState: StripeLoaderState = {
  dataSourceStatus: 'loading',
  data: undefined,
  error: undefined,
};

/**
 * StripeLoader retrieves billing information including the Stripe public key, and renders wrapped in a Stripe provider
 * @param {DataSource} dataSource - is the cloud service endpoint to request data from
 * @param {ChildRenderer} render - renders the child with the billing information and reload props
 * @param {() => React.ReactNode} errorRender - renders the error view
 * @param {() => React.ReactNode} loadingRender - renders the loading view
 */
export const StripeLoader = ({
  dataSource,
  render,
  errorRender,
  loadingRender,
}: StripeLoaderProps): React.ReactElement | null => {
  const ctx = useTeleport();
  const theme = useTheme();
  const [loaderState, setLoaderState] = useState(initialState);

  const loadData = (): void => {
    setLoaderState(initialState);

    dataSource(ctx.cloudService)
      .then((data: Response) => {
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

  useEffect(loadData, [render, ctx, dataSource]);

  switch (loaderState.dataSourceStatus) {
    case 'loading':
      if (loadingRender) {
        return (
          <Box textAlign="center" m={10}>
            {loadingRender()}
          </Box>
        );
      }
      // if no loading child is provided, don't expose to user
      return null;
    case 'error':
      if (errorRender) {
        return <>{errorRender(loaderState.error)}</>;
      }
      // if no error child is provided, don't expose to user
      return null;
    case 'success':
      return (
        <Elements
          key={`stripe-element-${loaderState.data.stripeCustomerId}`}
          stripe={loadStripe(loaderState.data.stripePublicKey)}
          options={{
            mode: 'setup',
            currency: 'usd',
            paymentMethodTypes: ['card'],
            appearance: getAppearance(theme),
          }}
        >
          {render(loaderState.data, loadData)}
        </Elements>
      );
  }
};
