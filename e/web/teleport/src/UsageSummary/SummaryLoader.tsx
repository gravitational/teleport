import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import { Alert, Box, Flex, Indicator, Text } from 'design';
import { Info } from 'design/Icon';
import useAttempt from 'shared/hooks/useAttemptNext';

import { UsageSummary } from 'e-teleport/services/cloud/v1/tenants_pb';
import { SummaryPage } from 'e-teleport/UsageSummary/SummaryPage';
import useTeleport from 'e-teleport/useTeleportE';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
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

  return (
    <Box>
      <StyledContainer>
        <FeatureBox>
          <Box>
            <FeatureHeader alignItems="center">
              <FeatureHeaderTitle mr="8">Billing Summary</FeatureHeaderTitle>
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
              <SummaryPage summary={usageSummary} />
            )}
          </Box>
        </FeatureBox>
        <InfoGuide />
      </StyledContainer>
    </Box>
  );
};

const StyledContainer = styled(Flex)`
  width: 100%;
  height: 100%;
  gap: ${({ theme }) => theme.space[5]}px;
  flex-direction: column;

  @media screen and (min-width: ${p => p.theme.breakpoints.medium}px) {
    align-items: center;
    flex-direction: row;
  }
`;

function InfoGuide() {
  return (
    <InfoGuideContainer>
      <Flex alignItems="flex-start" mb="3">
        <Info mr="2" /> <Text bold>Info Guide</Text>
      </Flex>
      <InfoTitle>How are Monthly Active Users (MAU) calculated?</InfoTitle>
      <InfoParagraph>
        Monthly Active Users (MAU) is the aggregate number of unique active
        users accessing Teleport during each monthly billing cycle.
      </InfoParagraph>
      <InfoTitle>
        How are Teleport Protected Resources (TPR) calculated?
      </InfoTitle>
      <InfoParagraph>
        Teleport Protected Resources (TPR) is an averaged aggregate number of
        unique resources connected to Teleport.
      </InfoParagraph>
      <InfoParagraph mt="2">
        A "resource" is any unique bot, such as a CI/CD Jenkins or GitHub
        Actions job, or a distinct computing resource, including a Kubernetes
        cluster, SSH server, database instance, or serverless endpoint, that
        registers with the Teleport cluster at least once a month.
      </InfoParagraph>
      <InfoParagraph mt="2">
        TPR is calculated by aggregating the total number of unique resources
        during the span of each hour in the day, and averaging the hourly count
        to create a daily TPR. The daily TPRs are then averaged across each
        billing period.
      </InfoParagraph>
    </InfoGuideContainer>
  );
}

const InfoGuideContainer = styled(Flex)`
  flex-direction: column;
  padding-left: ${({ theme }) => theme.space[6]}px;
  @media screen and (min-width: ${p => p.theme.breakpoints.medium}px) {
    border: none;
    border-left: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
    width: 400px;
    height: 100%;
    padding: ${({ theme }) => theme.space[3]}px;
  }
`;

const InfoTitle = styled(Text)`
  font-weight: 700;
  margin-bottom: ${p => p.theme.space[1]}px;
`;

const InfoParagraph = styled(Text)`
  font-weight: 200;
  margin-bottom: ${p => p.theme.space[3]}px;
`;
