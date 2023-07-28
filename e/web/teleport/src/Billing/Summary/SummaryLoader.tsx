import React, { useEffect, useState } from 'react';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import { Alert, Box, Indicator } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';

import { StripeLoader } from 'e-teleport/Billing/StripeLoader/StripeLoader';
import { SummaryPage } from 'e-teleport/Billing/Summary/SummaryPage';
import { ErrorPage } from 'e-teleport/Billing/common/ErrorPage';
import { LoadingPage } from 'e-teleport/Billing/common/LoadingPage';
import {
  BillingSummaryInformation,
  NonBillableSummaryInformation,
} from 'e-teleport/services/cloud';
import useTeleport from 'e-teleport/useTeleportE';

export const SummaryLoader = (): React.ReactElement => {
  const ctx = useTeleport();
  const { attempt, run } = useAttempt('processing');
  const [nonBillableUsage, setNonBillableUsage] =
    useState<NonBillableSummaryInformation>(null);

  useEffect(() => {
    run(() =>
      ctx.cloudService
        .fetchNonBillableSummaryInformation()
        .then(nonBillableData => {
          setNonBillableUsage(nonBillableData);
        })
    );
  }, [run, ctx.cloudService]);

  return (
    <FeatureBox mr="10">
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="8">Billing Summary</FeatureHeaderTitle>
      </FeatureHeader>

      {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}

      {attempt.status === 'success' && (
        <StripeLoader
          key={'stripe-summary'}
          dataSource={(CloudService): Promise<BillingSummaryInformation> =>
            CloudService.fetchBillingSummaryInformation()
          }
          render={(
            data: BillingSummaryInformation,
            reload: () => void
          ): React.ReactNode => (
            <SummaryPage
              data={data}
              nonBillableUsage={nonBillableUsage}
              reload={reload}
            />
          )}
          errorRender={(error: Error): React.ReactNode => (
            <ErrorPage message={error.message.toString()} />
          )}
          loadingRender={LoadingPage}
        />
      )}
    </FeatureBox>
  );
};
