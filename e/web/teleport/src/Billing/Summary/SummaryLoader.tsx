import React from 'react';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import { StripeLoader } from 'e-teleport/Billing/StripeLoader/StripeLoader';
import { SummaryPage } from 'e-teleport/Billing/Summary/SummaryPage';
import { ErrorPage } from 'e-teleport/Billing/common/ErrorPage';
import { LoadingPage } from 'e-teleport/Billing/common/LoadingPage';
import { BillingSummaryInformation } from 'e-teleport/services/cloud';

export const SummaryLoader = (): React.ReactElement => {
  return (
    <FeatureBox mr="10">
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="8">Billing</FeatureHeaderTitle>
      </FeatureHeader>

      <StripeLoader
        key={'stripe-summary'}
        dataSource={(CloudService): Promise<BillingSummaryInformation> =>
          CloudService.fetchBillingSummaryInformation()
        }
        render={(
          data: BillingSummaryInformation,
          reload: () => void
        ): React.ReactNode => <SummaryPage data={data} reload={reload} />}
        errorRender={(error: Error): React.ReactNode => (
          <ErrorPage message={error.message.toString()} />
        )}
        loadingRender={LoadingPage}
      />
    </FeatureBox>
  );
};
