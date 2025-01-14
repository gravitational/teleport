import { fromUnixTime } from 'date-fns';
import styled from 'styled-components';

import { Info } from 'design/Alert';
import Table, { Cell } from 'design/DataTable';
import Flex from 'design/Flex';
import { H2 } from 'design/Text';

import { UsageHistoryItem } from 'e-teleport/services/cloud/v1/tenants_pb';

export type UsageHistoryProps = {
  history: UsageHistoryItem[];
  maxMau: number;
  maxTpr: number;
};

export const UsageHistory = ({
  history,
  maxMau,
  maxTpr,
}: UsageHistoryProps) => {
  return (
    <Flex flexDirection="column" gap="3">
      <H2>Usage History</H2>
      <Info>
        Your current contract limit is set for {maxMau} MAU and {maxTpr} TPR.
      </Info>
      <Table<UsageHistoryItem>
        disableFilter={true}
        emptyText="No cycle information available"
        data={history}
        initialSort={{ altSortKey: 'cycleStart', dir: 'DESC' }}
        columns={[
          {
            headerText: 'Billing Cycle',
            render: ({ cycleStartFormatted, cycleEndFormatted, cycleEnd }) => (
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
            render: ({ mau, cycleEnd }) => (
              <StyledCell $highlight={isCurrentCycle(cycleEnd)}>
                {mau}
              </StyledCell>
            ),
          },
          {
            key: 'tpr',
            isSortable: true,
            headerText: 'Teleport Protected Resources (TPR)',
            render: ({ tpr, cycleEnd }) => (
              <StyledCell $highlight={isCurrentCycle(cycleEnd)}>
                {tpr}
              </StyledCell>
            ),
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
