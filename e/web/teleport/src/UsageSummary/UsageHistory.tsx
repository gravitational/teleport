import { fromUnixTime } from 'date-fns';
import styled from 'styled-components';

import Table, { Cell } from 'design/DataTable';
import Flex from 'design/Flex';
import Text, { H2 } from 'design/Text';

import { UsageHistoryItem } from 'e-teleport/services/cloud/v1/tenants_pb';

import { isCalibrationPeriod } from './SummaryPage';

export type UsageHistoryProps = {
  cloud: boolean;
  history: UsageHistoryItem[];
  hasCloudAnonymizationKey: boolean;
  salesforceIdUpdatedAt: number;
  hasIdentityGovernance: boolean;
  hasIdentitySecurity: boolean;
};

export const UsageHistory = ({
  cloud,
  history,
  hasCloudAnonymizationKey,
  salesforceIdUpdatedAt,
  hasIdentityGovernance,
  hasIdentitySecurity,
}: UsageHistoryProps) => {
  const tableHasCalibrationPeriod =
    history.length > 0 &&
    isCalibrationPeriod(
      cloud,
      history[history.length - 1].cycleStart,
      history[0].cycleEnd,
      hasCloudAnonymizationKey,
      salesforceIdUpdatedAt
    );

  return (
    <Flex flexDirection="column" gap="3">
      <H2 mb="4">Usage History</H2>
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
            headerText: 'ZTA MAU',
            render: ({ mau, cycleStart, cycleEnd }) => {
              const isCalibration = isCalibrationPeriod(
                cloud,
                cycleStart,
                cycleEnd,
                hasCloudAnonymizationKey,
                salesforceIdUpdatedAt
              );
              return (
                <MetricCell
                  val={mau}
                  cycleEnd={cycleEnd}
                  isCalibration={isCalibration}
                />
              );
            },
          },
          {
            key: 'tpr',
            isSortable: true,
            headerText: 'ZTA TPR',
            render: ({ tpr, cycleStart, cycleEnd }) => {
              const isCalibration = isCalibrationPeriod(
                cloud,
                cycleStart,
                cycleEnd,
                hasCloudAnonymizationKey,
                salesforceIdUpdatedAt
              );
              return (
                <MetricCell
                  val={tpr}
                  cycleEnd={cycleEnd}
                  isCalibration={isCalibration}
                />
              );
            },
          },
          {
            key: 'mwi',
            isSortable: true,
            headerText: 'MWI',
            render: ({ mwi, cycleStart, cycleEnd }) => {
              const isCalibration = isCalibrationPeriod(
                cloud,
                cycleStart,
                cycleEnd,
                hasCloudAnonymizationKey,
                salesforceIdUpdatedAt
              );
              return (
                <MetricCell
                  val={mwi}
                  cycleEnd={cycleEnd}
                  isCalibration={isCalibration}
                />
              );
            },
          },
          {
            key: 'igmau',
            isSortable: true,
            headerText: 'IG MAU',
            render: ({ igmau, cycleStart, cycleEnd }) => {
              const isCalibration = isCalibrationPeriod(
                cloud,
                cycleStart,
                cycleEnd,
                hasCloudAnonymizationKey,
                salesforceIdUpdatedAt
              );
              return (
                <MetricCell
                  val={igmau}
                  cycleEnd={cycleEnd}
                  isCalibration={isCalibration}
                  isDisabled={!hasIdentityGovernance}
                />
              );
            },
          },
          {
            altKey: 'is-tpr',
            isSortable: true,
            headerText: 'IS TPR',
            render: ({ tpr, cycleStart, cycleEnd }) => {
              const isCalibration = isCalibrationPeriod(
                cloud,
                cycleStart,
                cycleEnd,
                hasCloudAnonymizationKey,
                salesforceIdUpdatedAt
              );
              return (
                <MetricCell
                  val={tpr}
                  cycleEnd={cycleEnd}
                  isCalibration={isCalibration}
                  isDisabled={!hasIdentitySecurity}
                />
              );
            },
          },
        ]}
      />
      {tableHasCalibrationPeriod && (
        <Text
          css="font-style: italic"
          fontWeight={400}
          color="text.slightlyMuted"
        >
          * A change to your account required a calibration period in order to
          accurately count Active Users across trusted clusters.
        </Text>
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
  return fromUnixTime(cycleEnd).getTime() > now.getTime();
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
        '-'
      ) : isCalibration ? (
        <Text css="font-style: italic">Calibration Period*</Text>
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
