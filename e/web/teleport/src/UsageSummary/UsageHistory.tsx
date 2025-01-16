import { fromUnixTime } from 'date-fns';
import styled from 'styled-components';

import Table, { Cell } from 'design/DataTable';
import Flex from 'design/Flex';
import Text, { H2 } from 'design/Text';

import { UsageHistoryItem } from 'e-teleport/services/cloud/v1/tenants_pb';

import { isCalibrationPeriod } from './SummaryPage';

export type UsageHistoryProps = {
  history: UsageHistoryItem[];
  hasCloudAnonymizationKey: boolean;
  salesforceIdUpdatedAt: number;
};

export const UsageHistory = ({
  history,
  hasCloudAnonymizationKey,
  salesforceIdUpdatedAt,
}: UsageHistoryProps) => {
  return (
    <Flex flexDirection="column" gap="3">
      <H2>Usage History</H2>
      <Table<UsageHistoryItem>
        disableFilter={true}
        emptyText="No cycle information available"
        data={history}
        initialSort={{ altSortKey: 'cycleStart', dir: 'DESC' }}
        columns={[
          {
            headerText: 'Billing Cycle',
            render: ({ cycleStartFormatted, cycleEnd, cycleEndFormatted }) => (
              <StyledCell $highlight={isCurrentCycle(cycleEnd)}>
                {cycleStartFormatted} - {cycleEndFormatted}
              </StyledCell>
            ),
            isSortable: true,
            key: 'cycleStart',
          },
          {
            key: 'mau',
            isSortable: true,
            headerText: 'Monthly Active Users (MAU)',
            render: ({ mau, cycleStart, cycleEnd }) => {
              const isCalibration = isCalibrationPeriod(
                cycleStart,
                cycleEnd,
                hasCloudAnonymizationKey,
                salesforceIdUpdatedAt
              );
              return (
                <StyledCell $highlight={isCurrentCycle(cycleEnd)}>
                  {isCalibration ? (
                    <Text css="font-style: italic">Calibration Period</Text>
                  ) : (
                    mau
                  )}
                </StyledCell>
              );
            },
          },
          {
            key: 'tpr',
            isSortable: true,
            headerText: 'Teleport Protected Resources (TPR)',
            render: ({ tpr, cycleStart, cycleEnd }) => {
              const isCalibration = isCalibrationPeriod(
                cycleStart,
                cycleEnd,
                hasCloudAnonymizationKey,
                salesforceIdUpdatedAt
              );
              return (
                <StyledCell $highlight={isCurrentCycle(cycleEnd)}>
                  {isCalibration ? (
                    <Text css="font-style: italic">Calibration Period</Text>
                  ) : (
                    tpr
                  )}
                </StyledCell>
              );
            },
          },
        ]}
      />
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
  return fromUnixTime(cycleEnd).getTime() > now.getTime();
};

const StyledCell = styled(Cell)<{ $highlight: boolean }>`
  background-color: ${({ $highlight, theme }) =>
    $highlight ? theme.colors.levels.surface : 'unset'};
`;
