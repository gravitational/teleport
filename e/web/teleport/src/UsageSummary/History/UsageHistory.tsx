import styled from 'styled-components';

import { Flex, H2, P2, Text } from 'design';
import Table, { Cell } from 'design/DataTable';
import { TableColumn } from 'design/DataTable/types';

import {
  GetUsageResponse,
  PricingModel,
  UsageCycle,
} from 'e-teleport/services/cloud/v1/tenants_pb';
import { usageUnixInMilliseconds } from 'e-teleport/UsageSummary/helpers';

export type UsageHistoryProps = {
  usageResponse: GetUsageResponse;
  isV1Pricing: boolean;
};

enum Metric {
  MAU = 'MAU',
  TPR = 'TPR',
  IGMAU = 'IGMAU',
  MWI = 'MWI',
  ISTPR = 'ISTPR',
}

export const UsageHistory = ({
  usageResponse,
  isV1Pricing,
}: UsageHistoryProps) => {
  const cols: TableColumn<UsageCycle>[] = [
    {
      headerText: 'Cycle',
      render: ({ startFormatted, endFormatted, end }) => (
        <StyledCell $highlight={isCurrentCycle(end)} style={{ minWidth: 230 }}>
          <Flex gap="2">
            {startFormatted} - {endFormatted}{' '}
            {isCurrentCycle(end) && (
              <Text color="text.slightlyMuted" fontWeight="700">
                (Current)
              </Text>
            )}
          </Flex>
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
  ];

  if (isV1Pricing) {
    cols.push({
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
    });
    cols.push({
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
    });

    cols.push({
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
    });
  }

  return (
    <Flex flexDirection="column" gap="3">
      <H2 mb="4">Usage Reporting History</H2>
      <Table<UsageCycle>
        disableFilter={true}
        emptyText="No cycle information available"
        data={usageResponse.usageHistory}
        initialSort={{ altSortKey: 'start', dir: 'DESC' }}
        columns={cols}
      />
      {usageResponse.usageHistory.some(h => h.calibratingAccounts > 0) && (
        <P2 color="text.muted">
          * A calibration period appears for any cycle with usage limit changes
          to ensure accurate active user counts across trusted clusters.
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
      return;
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
