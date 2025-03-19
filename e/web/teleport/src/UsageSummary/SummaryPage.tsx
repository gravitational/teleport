import styled from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';

import { UsageSummary } from 'e-teleport/services/cloud/v1/tenants_pb';
import { Cycle } from 'e-teleport/UsageSummary/Cycle';

import { UsageHistory } from './UsageHistory';

export interface SummaryProps {
  summary: UsageSummary;
}

export const SummaryPage = ({ summary }: SummaryProps) => (
  <>
    {summary ? (
      <Flex gap="5" flexDirection="column">
        <Cycle summary={summary} />
        <UsageHistory
          history={summary.usageHistory}
          hasCloudAnonymizationKey={summary.hasCloudAnonymizationKey}
          salesforceIdUpdatedAt={summary.salesforceIdUpdatedAt}
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
 * @param cycleStart unix timestamp of the start of the cycle
 * @param cycleEnd unix timestamp of the end of the cycle
 * @param hasCloudAnonymizationKey whether the account has a Cloud Anonymization key or not
 * @param salesforceIdUpdatedAt unix timestamp of when the SFID of the account was updated
 * @returns true if the cycle is a calibration period
 */
export const isCalibrationPeriod = (
  cycleStart: number,
  cycleEnd: number,
  hasCloudAnonymizationKey: boolean,
  salesforceIdUpdatedAt: number
): boolean => {
  return (
    hasCloudAnonymizationKey &&
    salesforceIdUpdatedAt <= cycleEnd &&
    salesforceIdUpdatedAt >= cycleStart
  );
};
