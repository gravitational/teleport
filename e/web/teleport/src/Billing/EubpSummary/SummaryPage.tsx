import React from 'react';

import styled from 'styled-components';

import Box from 'design/Box';

import { Cycle } from 'e-teleport/Billing/EubpSummary/Cycle';
import { BillingSummaryInformation } from 'e-teleport/services/cloud';

export interface SummaryProps {
  data: BillingSummaryInformation;
}

export const SummaryPage = ({
  data: {
    productName,
    stripeCurrentUsage,
    stripeMissingPaymentMethod,
    stripeTrialEnd,
    usageUpdatedAt,
    usageQuota,
  },
}: SummaryProps) => (
  <>
    {stripeCurrentUsage ? (
      <Cycle
        currentUsage={stripeCurrentUsage}
        productName={productName}
        stripeMissingPaymentMethod={stripeMissingPaymentMethod}
        stripeTrialEnd={stripeTrialEnd}
        usageUpdatedAt={usageUpdatedAt}
        usageQuota={usageQuota}
      />
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
