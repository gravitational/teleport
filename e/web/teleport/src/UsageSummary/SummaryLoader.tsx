import React, { useEffect, useState } from 'react';

import { Alert, Box, Indicator } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';

import { UsageSummary } from 'e-teleport/services/cloud/v1/tenants_pb';
import { SummaryPage } from 'e-teleport/UsageSummary/SummaryPage';
import useTeleport from 'e-teleport/useTeleportE';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

export const SummaryLoader = (): React.ReactElement => {
  const ctx = useTeleport();
  const { attempt, run } = useAttempt('processing');
  const [usageSummary, setUsageSummary] = useState<UsageSummary>(null);

  useEffect(() => {
    run(() =>
      ctx.cloudService
        .fetchBillingSummaryInformation()
        .then(data => setUsageSummary(data.usageSummary))
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

      {attempt.status === 'success' && <SummaryPage summary={usageSummary} />}
    </FeatureBox>
  );
};
