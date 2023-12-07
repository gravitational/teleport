import React, { useEffect, useState } from 'react';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import { Alert, Box, Indicator } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';

import { SummaryPage } from 'e-teleport/Billing/EubpSummary/SummaryPage';
import { BillingSummaryInformation } from 'e-teleport/services/cloud';
import useTeleport from 'e-teleport/useTeleportE';

export const SummaryLoader = (): React.ReactElement => {
  const ctx = useTeleport();
  const { attempt, run } = useAttempt('processing');
  const [billingSummary, setBillingSummary] =
    useState<BillingSummaryInformation>(null);

  useEffect(() => {
    run(() =>
      ctx.cloudService
        .fetchBillingSummaryInformation()
        .then(bililngSummaryData => setBillingSummary(bililngSummaryData))
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

      {attempt.status === 'success' && <SummaryPage data={billingSummary} />}
    </FeatureBox>
  );
};
