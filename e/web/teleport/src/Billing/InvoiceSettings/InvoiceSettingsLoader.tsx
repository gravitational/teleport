import React from 'react';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import { StripeLoader } from 'e-teleport/Billing/StripeLoader/StripeLoader';
import { InvoiceSettingsInformation } from 'e-teleport/services/cloud';
import { ErrorPage } from 'e-teleport/Billing/common/ErrorPage';
import { LoadingPage } from 'e-teleport/Billing/common/LoadingPage';
import { InvoiceSettingsPage } from 'e-teleport/Billing/InvoiceSettings/InvoiceSettingsPage';

export const InvoiceSettingsLoader = () => {
  return (
    <FeatureBox mr="10">
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="8">Invoice Settings</FeatureHeaderTitle>
      </FeatureHeader>

      <StripeLoader
        key={'stripe-invoice-settings'}
        dataSource={(CloudService): Promise<InvoiceSettingsInformation> =>
          CloudService.fetchInvoiceSettings()
        }
        render={(
          data: InvoiceSettingsInformation,
          reload: () => void
        ): React.ReactNode => (
          <InvoiceSettingsPage data={data} reload={reload} />
        )}
        errorRender={(error: Error): React.ReactNode => (
          <ErrorPage message={error.message.toString()} />
        )}
        loadingRender={LoadingPage}
      />
    </FeatureBox>
  );
};
