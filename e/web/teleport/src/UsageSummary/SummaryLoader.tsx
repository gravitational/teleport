import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import { Alert, Box, Flex, Indicator } from 'design';
import {
  InfoGuideButton,
  InfoParagraph,
  InfoTitle,
  ReferenceLinks,
} from 'shared/components/SlidingSidePanel/InfoGuide';
import useAttempt from 'shared/hooks/useAttemptNext';

import { UsageSummary } from 'e-teleport/services/cloud/v1/tenants_pb';
import { SummaryPage } from 'e-teleport/UsageSummary/SummaryPage';
import useTeleport from 'e-teleport/useTeleportE';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { useNoMinWidth } from 'teleport/Main';

export const SummaryLoader = (): React.ReactElement => {
  const ctx = useTeleport();
  const { attempt, run } = useAttempt('processing');
  const [usageSummary, setUsageSummary] = useState<UsageSummary>(null);

  useNoMinWidth();

  useEffect(() => {
    run(() =>
      ctx.cloudService
        .fetchBillingSummaryInformation()
        .then(data => setUsageSummary(data.usageSummary))
    );
  }, [run, ctx.cloudService]);

  const hasIdentityGovernance = cfg.entitlements.Identity.enabled;
  const hasIdentitySecurity = cfg.entitlements.Policy.enabled;

  return (
    <Box>
      <StyledContainer>
        <FeatureBox>
          <Box>
            <FeatureHeader alignItems="center" justifyContent="space-between">
              <FeatureHeaderTitle>Usage Reporting</FeatureHeaderTitle>
              <InfoGuideButton config={{ guide: <InfoGuide /> }} />
            </FeatureHeader>

            {attempt.status === 'failed' && (
              <Alert children={attempt.statusText} />
            )}
            {attempt.status === 'processing' && (
              <Box textAlign="center" m={10}>
                <Indicator />
              </Box>
            )}

            {attempt.status === 'success' && (
              <SummaryPage
                summary={usageSummary}
                hasIdentityGovernance={hasIdentityGovernance}
                hasIdentitySecurity={hasIdentitySecurity}
              />
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

function InfoGuide() {
  return (
    <Box>
      <InfoTitle>How Monthly Active Users (MAU) are Calculated</InfoTitle>
      <InfoParagraph>
        Monthly Active Users (MAU) is the aggregate number of unique active
        users accessing Teleport during each monthly billing cycle.
      </InfoParagraph>
      <InfoTitle>
        How Teleport Protected Resources (TPR) are Calculated
      </InfoTitle>
      <InfoParagraph>
        Teleport Protected Resources (TPR) is an averaged aggregate number of
        unique resources connected to Teleport.
      </InfoParagraph>
      <InfoParagraph>
        A &quot;resource&quot; is any unique bot, such as a CI/CD Jenkins or
        GitHub Actions job, or a distinct computing resource, including a
        Kubernetes cluster, SSH server, database instance, or serverless
        endpoint, that registers with the Teleport cluster at least once a
        month.
      </InfoParagraph>
      <InfoParagraph>
        TPR is calculated by aggregating the total number of unique resources
        during the span of each hour in the day, and averaging the hourly count
        to create a daily TPR. The daily TPRs are then averaged across each
        billing period.
      </InfoParagraph>
      <InfoTitle>
        How Machine and Workload Identities (MWI) are Calculated
      </InfoTitle>
      <InfoParagraph>
        Machine and Workload Identities is an aggregate number of bots, bot
        instances, and SPIFFE IDs.
      </InfoParagraph>
      <InfoParagraph>
        MWIs are calculated by counting the total number of bots, bot instances,
        and unique SPIFFE IDs seen in an hour and averaging the hourly number to
        create a daily average. The daily MWI numbers are then averaged across
        each billing period.
      </InfoParagraph>
      <ReferenceLinks
        links={[
          {
            title: 'Usage Reporting and Billing',
            href: 'https://goteleport.com/docs/usage-billing/',
          },
        ]}
      />
    </Box>
  );
}
