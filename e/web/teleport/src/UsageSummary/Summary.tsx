import { useQuery } from '@tanstack/react-query';
import React from 'react';
import styled from 'styled-components';

import { Box, Flex, Indicator } from 'design';
import { Danger } from 'design/Alert';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';

import { Cycle } from 'e-teleport/UsageSummary/Cycle/Cycle';
import { UsageHistory } from 'e-teleport/UsageSummary/History/UsageHistory';
import useTeleport from 'e-teleport/useTeleportE';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { useNoMinWidth } from 'teleport/Main';

import { Guide } from './Guide';

export const Summary = (): React.ReactElement => {
  useNoMinWidth();
  const ctx = useTeleport();
  const hasIdentityGovernance = cfg.entitlements.Identity.enabled;
  const hasIdentitySecurity = cfg.entitlements.Policy.enabled;

  const {
    data: summary,
    error,
    status,
  } = useQuery({
    queryKey: ['summary'],
    gcTime: 0,
    queryFn: () =>
      ctx.cloudService.fetchBillingSummaryInformation().then(data => data),
  });

  return (
    <Box>
      <StyledContainer>
        <FeatureBox>
          <Box>
            <FeatureHeader alignItems="center" justifyContent="space-between">
              <FeatureHeaderTitle>Usage Reporting</FeatureHeaderTitle>
              <InfoGuideButton config={{ guide: <Guide /> }} />
            </FeatureHeader>
            {status === 'error' && error && (
              <Danger details={error.message}>Error: {error.name}</Danger>
            )}
            {status === 'pending' && (
              <Box textAlign="center" m={10}>
                <Indicator />
              </Box>
            )}

            {status === 'success' && summary && summary.usageSummary && (
              <Flex gap="5" flexDirection="column">
                <Cycle
                  summary={summary?.usageSummary}
                  hasIdentityGovernance={hasIdentityGovernance}
                  hasIdentitySecurity={hasIdentitySecurity}
                />
                <UsageHistory
                  cloud={summary?.usageSummary?.cloud}
                  history={summary?.usageSummary.usageHistory}
                  hasCloudAnonymizationKey={
                    summary?.usageSummary.hasCloudAnonymizationKey
                  }
                  salesforceIdUpdatedAt={summary.salesforceIdUpdatedAt}
                  hasIdentityGovernance={hasIdentityGovernance}
                  hasIdentitySecurity={hasIdentitySecurity}
                />
              </Flex>
            )}
            {status === 'success' && !summary && (
              <StyledBox>
                Usage data is being gathered. This page updates every 12 hours.
              </StyledBox>
            )}
          </Box>
        </FeatureBox>
      </StyledContainer>
    </Box>
  );
};

const StyledContainer = styled(Flex)`
  width: 100%;
  height: 100%;
  gap: ${({ theme }) => theme.space[5]}px;
  flex-direction: column;

  @media screen and (min-width: ${p => p.theme.breakpoints.medium}) {
    align-items: center;
    flex-direction: row;
  }
`;

const StyledBox = styled(Box)`
  background: ${({ theme }) => theme.colors.levels.surface};
  border-radius: 8px;
  margin: 20px 0 0;
  padding: 20px 0 20px 40px;
`;
