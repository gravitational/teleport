import styled from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';

import { UsageSummary } from 'e-teleport/services/cloud/v1/tenants_pb';
import { Cycle } from 'e-teleport/UsageSummary/Cycle';

import { UsageHistory } from './UsageHistory';

export interface SummaryProps {
  summary: UsageSummary;
  hasIdentityGovernance: boolean;
  hasIdentitySecurity: boolean;
}

export const SummaryPage = ({
  summary,
  hasIdentityGovernance,
  hasIdentitySecurity,
}: SummaryProps) => (
  <>
    {summary ? (
      <Flex gap="5" flexDirection="column">
        <Cycle
          summary={summary}
          hasIdentityGovernance={hasIdentityGovernance}
          hasIdentitySecurity={hasIdentitySecurity}
        />
        <UsageHistory
          cloud={summary.cloud}
          history={summary.usageHistory}
          hasCloudAnonymizationKey={summary.hasCloudAnonymizationKey}
          salesforceIdUpdatedAt={summary.salesforceIdUpdatedAt}
          hasIdentityGovernance={hasIdentityGovernance}
          hasIdentitySecurity={hasIdentitySecurity}
        />
      </Flex>
    ) : (
      <StyledBox>
        Usage data is being gathered. This page updates every 12 hours.
      </StyledBox>
    )}
  </>
);

const StyledBox = styled(Box)`
  background: ${({ theme }) => theme.colors.levels.surface};
  border-radius: 8px;
  margin: 20px 0 0;
  padding: 20px 0 20px 40px;
`;

/**
 *
 * @param cloud boolean value indicating customer is on a Teleport Cloud subscription
 * @param cycleStart unix timestamp of the start of the cycle
 * @param cycleEnd unix timestamp of the end of the cycle
 * @param hasCloudAnonymizationKey whether the account has a Cloud Anonymization key or not
 * @param salesforceIdUpdatedAt unix timestamp of when the SFID of the account was updated
 * @returns true if the cycle is a calibration period
 */
export const isCalibrationPeriod = (
  cloud: boolean,
  cycleStart: number,
  cycleEnd: number,
  hasCloudAnonymizationKey: boolean,
  salesforceIdUpdatedAt: number
): boolean => {
  return (
    cloud &&
    hasCloudAnonymizationKey &&
    salesforceIdUpdatedAt <= cycleEnd &&
    salesforceIdUpdatedAt >= cycleStart
  );
};
