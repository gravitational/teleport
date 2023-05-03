import React from 'react';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import { PaymentsInvoicesInformation } from 'e-teleport/services/cloud';
import { PaymentsAndInvoicesPage } from 'e-teleport/Billing/PaymentsAndInvoices/PaymentsAndInvoicesPage';
import { ErrorPage } from 'e-teleport/Billing/common/ErrorPage';
import { LoadingPage } from 'e-teleport/Billing/common/LoadingPage';
import { StripeLoader } from 'e-teleport/Billing/StripeLoader/StripeLoader';

export const PaymentsAndInvoicesLoader = (): React.ReactElement => {
  return (
    <FeatureBox mr="10">
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="8">Payments and Invoices</FeatureHeaderTitle>
      </FeatureHeader>

      <StripeLoader
        key={'stripe-payments-and-invoices'}
        dataSource={(CloudService): Promise<PaymentsInvoicesInformation> =>
          CloudService.fetchPaymentsAndInvoices()
        }
        render={(
          data: PaymentsInvoicesInformation,
          reload: () => void
        ): React.ReactNode => (
          <PaymentsAndInvoicesPage data={data} reload={reload} />
        )}
        errorRender={(error: Error): React.ReactNode => (
          <ErrorPage message={error.message.toString()} />
        )}
        loadingRender={LoadingPage}
      />
    </FeatureBox>
  );
};
