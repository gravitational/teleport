import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import styled from 'styled-components';

import { Alert, Box, ButtonSelect, Flex } from 'design';
import { Danger } from 'design/Alert';
import { AlertKind } from 'design/Alert/Alert';
import { ShimmerBox } from 'design/ShimmerBox';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';

import { GetUsageResponse } from 'e-teleport/services/cloud/v1/tenants_pb';
import { Cycle } from 'e-teleport/UsageSummary/Cycle/Cycle';
import { UsageHistory } from 'e-teleport/UsageSummary/History/UsageHistory';
import useTeleport from 'e-teleport/useTeleportE';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { useNoMinWidth } from 'teleport/Main';

import { Guide } from './Guide';

export function Summary() {
  useNoMinWidth();

  // Initial implementation for GetUsage allows for a nil UUID as a
  // representation of the accountId associated with the license of the
  // requestor. When sent, the cloud service will translate this as the
  // 'current' tenant.
  const nilId = '00000000-0000-0000-0000-000000000000';
  const ctx = useTeleport();
  const [aggregate, setAggregate] = useState(true);
  const [customer, setCustomer] = useState<GetUsageResponse>();

  const {
    data: usageResponse,
    error,
    status,
    isRefetching,
  } = useQuery({
    queryKey: ['usageResponse', aggregate],
    placeholderData: customer,
    queryFn: () => {
      const request = aggregate ? [] : [nilId];
      return ctx.cloudService
        .fetchBillingSummaryInformation({ tenants: request })
        .then(data => {
          // if we have not yet set the customer, and the request is customer
          // level, and the response indicates more than one subscription, set
          // customer values to be used in underlay for tenant view.
          if (!customer && request.length == 0 && data.aggregateCount > 1) {
            setCustomer(data);
          }
          return data;
        });
    },
  });

  return (
    <Box>
      <StyledContainer>
        <FeatureBox>
          {status == 'pending' && <ShimmerBox height="24px" width="100%" />}
          {status == 'error' && (
            <Danger details={error.message}>Error: {error.name}</Danger>
          )}
          {status == 'success' &&
            usageResponse &&
            usageResponse.usageHistory &&
            usageResponse.usageHistory.length > 0 && (
              <Box>
                <FeatureHeader
                  alignItems="center"
                  justifyContent="space-between"
                >
                  <FeatureHeaderTitle>Usage Reporting</FeatureHeaderTitle>
                  <Flex alignItems="center" gap={3}>
                    {usageResponse.aggregateCount > 1 && (
                      <ButtonSelect
                        fullWidth
                        disabled={isRefetching}
                        options={[
                          {
                            value: 'all',
                            label: `All Clusters (${usageResponse?.aggregateCount})`,
                          },
                          { value: 'current', label: 'Current Cluster' },
                        ]}
                        activeValue={aggregate ? 'all' : 'current'}
                        onChange={() => setAggregate(!aggregate)}
                      />
                    )}
                    <InfoGuideButton config={{ guide: <Guide /> }} />
                  </Flex>
                </FeatureHeader>
                <>
                  {usageResponse.alerts?.map((alert, i) => {
                    if (alert.name || alert.details) {
                      return (
                        <Alert
                          key={i}
                          kind={(alert.kind as AlertKind) || 'neutral'}
                          details={alert.details || ''}
                          dismissible={alert.dismissible || false}
                        >
                          {alert.name}
                        </Alert>
                      );
                    }
                  })}
                </>
                <Flex gap="5" flexDirection="column">
                  <Cycle
                    usageResponse={usageResponse}
                    customer={customer}
                    aggregate={aggregate}
                  />
                  <UsageHistory usageResponse={usageResponse} />
                </Flex>
              </Box>
            )}
          {status === 'success' &&
            (!usageResponse ||
              !usageResponse?.usageHistory ||
              usageResponse?.usageHistory?.length === 0) && (
              <StyledBox>
                Usage data is being gathered. This page updates every 12 hours.
              </StyledBox>
            )}
        </FeatureBox>
      </StyledContainer>
    </Box>
  );
}

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
