import styled from 'styled-components';

import Box from 'design/Box';

import { UsageSummary } from 'e-teleport/services/cloud/v1/tenants_pb';
import { Cycle } from 'e-teleport/UsageSummary/Cycle';

export interface SummaryProps {
  summary: UsageSummary;
}

export const SummaryPage = ({ summary }: SummaryProps) => (
  <>
    {summary ? (
      <Cycle summary={summary} />
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
