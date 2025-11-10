import styled from 'styled-components';

import { Flex, H2, P2, Text } from 'design';
import Table, { Cell } from 'design/DataTable';

import {
  GetUsageResponse,
  PricingModel,
  UsageCycle,
} from 'e-teleport/services/cloud/v1/tenants_pb';
import { usageUnixInMilliseconds } from 'e-teleport/UsageSummary/helpers';

export type UsageHistoryProps = {
  usageResponse: GetUsageResponse;
};

enum Metric {
  MAU = 'MAU',
  TPR = 'TPR',
  IGMAU = 'IGMAU',
  MWI = 'MWI',
  ISTPR = 'ISTPR',
}

export const UsageHistory = ({ usageResponse }: UsageHistoryProps) => {
  return (
    <Flex flexDirection="column" gap="3">
      <H2 mb="4">Usage Reporting History</H2>
      <Table<UsageCycle>
        disableFilter={true}
        emptyText="No cycle information available"
        data={usageResponse.usageHistory}
        initialSort={{ altSortKey: 'start', dir: 'DESC' }}
        columns={[
          {
            headerText: 'Usage Cycle',
            render: ({ startFormatted, endFormatted, end }) => (
              <StyledCell $highlight={isCurrentCycle(end)}>
                {startFormatted} - {endFormatted}
              </StyledCell>
            ),
            isSortable: true,
            key: 'start',
          },
          {
            altKey: 'usage.ztamau',
            isSortable: true,
            headerText: 'Monthly Active Users (MAU)',
            render: ({
              calibratingAccounts,
              usage,
              end,
              pricingModel,
            }: UsageCycle) => {
              return (
                <MetricCell
                  metric={Metric.MAU}
                  model={pricingModel}
                  val={usage.ztamau}
                  cycleEnd={end}
                  isCalibration={calibratingAccounts >= 1}
                />
              );
            },
          },
          {
            altKey: 'usage.tpr',
            isSortable: true,
            headerText: 'Teleport Protected Resources (TPR)',
            render: ({
              calibratingAccounts,
              usage,
              end,
              pricingModel,
            }: UsageCycle) => {
              return (
                <MetricCell
                  metric={Metric.TPR}
                  model={pricingModel}
                  val={usage.tpr}
                  cycleEnd={end}
                  isCalibration={calibratingAccounts >= 1}
                />
              );
            },
          },
          {
            altKey: 'usage.mwi',
            isSortable: true,
            headerText: 'Machine & Workload Identities',
            render: ({
              calibratingAccounts,
              usage,
              end,
              pricingModel,
            }: UsageCycle) => {
              return (
                <MetricCell
                  metric={Metric.MWI}
                  model={pricingModel}
                  val={usage.mwi}
                  cycleEnd={end}
                  isCalibration={calibratingAccounts >= 1}
                />
              );
            },
          },
          {
            altKey: 'usage.igmau',
            isSortable: true,
            headerText: 'Identity Governance MAU',
            render: ({
              calibratingAccounts,
              usage,
              end,
              pricingModel,
            }: UsageCycle) => {
              return (
                <MetricCell
                  metric={Metric.IGMAU}
                  model={pricingModel}
                  val={usage.igmau}
                  cycleEnd={end}
                  isCalibration={calibratingAccounts >= 1}
                  cta={usageResponse.missingEntitlements.includes('Identity')}
                />
              );
            },
          },
          {
            altKey: 'is-tpr',
            isSortable: true,
            headerText: 'Identity Security TPR',
            render: ({
              calibratingAccounts,
              usage,
              end,
              pricingModel,
            }: UsageCycle) => {
              return (
                <MetricCell
                  metric={Metric.ISTPR}
                  model={pricingModel}
                  val={usage.tpr}
                  cycleEnd={end}
                  isCalibration={calibratingAccounts >= 1}
                  cta={usageResponse.missingEntitlements.includes('Policy')}
                />
              );
            },
          },
        ]}
      />
      {usageResponse.usageHistory.some(h => h.calibratingAccounts > 0) && (
        <P2 color="text.slightlyMuted">
          * A change to your account required a calibration period in order to
          accurately count Active Users across trusted clusters.
        </P2>
      )}
    </Flex>
  );
};

/**
 * Checks if the current time is before the specified cycle end.
 * @param {number} cycleEnd - Unix timestamp for the cycle's end.
 * @returns {boolean} `true` if the cycle is ongoing (hasn't ended), otherwise `false`.
 */
const isCurrentCycle = (cycleEnd: number): boolean => {
  const now = new Date();
  return usageUnixInMilliseconds(cycleEnd) > now.getTime();
};

const MetricCell = ({
  model,
  metric,
  val,
  cycleEnd,
  isCalibration,
  cta,
}: {
  model: PricingModel;
  metric: Metric;
  val: number;
  cycleEnd: number;
  isCalibration: boolean;
  cta?: boolean;
}) => {
  function getContents() {
    if (!model.metric.includes(metric)) {
      return (
        <Text color="text.slightlyMuted" style={{ fontStyle: 'italic' }}>
          Not in use
        </Text>
      );
    }
    if (cta) {
      return (
        <Text color="text.slightlyMuted" style={{ fontStyle: 'italic' }}>
          Not available
        </Text>
      );
    }
    if (isCalibration) {
      return (
        <Text color="text.slightlyMuted" style={{ fontStyle: 'italic' }}>
          Calibration Period*
        </Text>
      );
    }

    return val;
  }

  return (
    <StyledCell $highlight={isCurrentCycle(cycleEnd)}>
      {getContents()}
    </StyledCell>
  );
};

const StyledCell = styled(Cell)<{ $highlight: boolean }>`
  background-color: ${({ $highlight, theme }) =>
    $highlight ? theme.colors.levels.surface : 'unset'};
`;
