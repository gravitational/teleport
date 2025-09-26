import styled from 'styled-components';

import Table, { Cell } from 'design/DataTable';
import Flex from 'design/Flex';
import Text, { H2, P2 } from 'design/Text';

import {
  GetUsageResponse,
  UsageCycle,
} from 'e-teleport/services/cloud/v1/tenants_pb';
import { usageUnixInMilliseconds } from 'e-teleport/UsageSummary/helpers';

export type UsageHistoryProps = {
  usageResponse: GetUsageResponse;
};

export const UsageHistory = ({ usageResponse }: UsageHistoryProps) => {
  return (
    <Flex flexDirection="column" gap="3">
      <H2 mb="4">Usage History</H2>
      <Table<UsageCycle>
        disableFilter={true}
        emptyText="No cycle information available"
        data={usageResponse.usageHistory}
        initialSort={{ altSortKey: 'start', dir: 'DESC' }}
        columns={[
          {
            headerText: 'Billing Cycle',
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
            headerText: 'ZTA MAU',
            render: ({ calibratingAccounts, usage, end }: UsageCycle) => {
              return (
                <MetricCell
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
            headerText: 'ZTA TPR',
            render: ({ calibratingAccounts, usage, end }) => {
              return (
                <MetricCell
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
            headerText: 'MWI',
            render: ({ calibratingAccounts, usage, end }) => {
              return (
                <MetricCell
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
            headerText: 'IG MAU',
            render: ({ calibratingAccounts, usage, end }) => {
              return (
                <MetricCell
                  val={usage.igmau}
                  cycleEnd={end}
                  isCalibration={calibratingAccounts >= 1}
                  isDisabled={usageResponse.missingEntitlements.includes(
                    'Identity'
                  )}
                />
              );
            },
          },
          {
            altKey: 'is-tpr',
            isSortable: true,
            headerText: 'IS TPR',
            render: ({ calibratingAccounts, usage, end }) => {
              return (
                <MetricCell
                  val={usage.tpr}
                  cycleEnd={end}
                  isCalibration={calibratingAccounts >= 1}
                  isDisabled={usageResponse.missingEntitlements.includes(
                    'Policy'
                  )}
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
  val,
  cycleEnd,
  isCalibration,
  isDisabled,
}: {
  val: number;
  cycleEnd: number;
  isCalibration: boolean;
  isDisabled?: boolean;
}) => {
  return (
    <StyledCell $highlight={isCurrentCycle(cycleEnd)}>
      {isDisabled ? (
        'N/A'
      ) : isCalibration ? (
        <Text style={{ fontStyle: 'italic' }}>Calibration Period*</Text>
      ) : (
        val
      )}
    </StyledCell>
  );
};

const StyledCell = styled(Cell)<{ $highlight: boolean }>`
  background-color: ${({ $highlight, theme }) =>
    $highlight ? theme.colors.levels.surface : 'unset'};
`;
