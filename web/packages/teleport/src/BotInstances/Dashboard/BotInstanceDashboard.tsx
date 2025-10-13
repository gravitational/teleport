/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useQuery } from '@tanstack/react-query';
import { format, formatDistanceToNowStrict, parseISO } from 'date-fns';
import styled, { useTheme } from 'styled-components';

import { Alert } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import ButtonIcon from 'design/ButtonIcon/ButtonIcon';
import { CardTile } from 'design/CardTile/CardTile';
import Flex from 'design/Flex';
import { Refresh } from 'design/Icon';
import { Indicator } from 'design/Indicator/Indicator';
import Text, { H2, H3 } from 'design/Text';
import { IconTooltip } from 'design/Tooltip';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';

import { getBotInstanceMetrics } from 'teleport/services/bot/bot';
import { GetBotInstanceMetricsResponse } from 'teleport/services/bot/types';

export function BotInstancesDashboard(props: {
  /**
   * Callback used when a dashbaord item is selected (e.g. "unsupported"
   * instance versions). The given filter is used as an advanced query (in the
   * Teleport predicate language) to filter the items in the instances list.
   *
   * @param filter query (verbatum) used to filter the bot instance list.
   */
  onFilterSelected: (filter: string) => void;
}) {
  const { onFilterSelected } = props;

  const { data, error, isLoading, isPending, refetch } = useQuery({
    queryKey: ['bot_instance', 'metrics'],
    queryFn: ({ signal }) => getBotInstanceMetrics(null, signal),
    // The metrics endpoint (used by this query) returns a
    // `refresh_after_seconds` value to indicate how frequently the client
    // should poll for updated metrics, which may take jitter into account. This
    // allows the polling rate to most closely match the backend data refresh,
    // and allows the rate to be controlled server-side.
    //
    // The `refetchInterval` is set to this value from the lasty successful
    // response, otherwise 1 min as a fallback.
    refetchInterval: ({ state }) =>
      (state.data?.refresh_after_seconds ?? 60) * 1_000,
  });

  return (
    <Container>
      <TitleContainer>
        <H2>Dashboard</H2>
        <HoverTooltip placement="top" tipContent={'Refresh metrics'}>
          <ButtonIcon
            onClick={() => refetch()}
            aria-label="refresh"
            disabled={isLoading}
          >
            <Refresh size="medium" />
          </ButtonIcon>
        </HoverTooltip>
      </TitleContainer>
      <Divider />

      {error ? (
        <Alert kind="danger" m={3}>
          {error.message}
        </Alert>
      ) : undefined}

      {isLoading ? (
        <Box data-testid="loading-dashboard" textAlign="center" m={10}>
          <Indicator />
        </Box>
      ) : undefined}

      {isPending ? undefined : (
        <InnerContainer>
          <UpgradeStatusChart
            data={data?.upgrade_statuses}
            onFilterSelected={onFilterSelected}
          />
        </InnerContainer>
      )}
    </Container>
  );
}

const Container = styled(CardTile)`
  flex-direction: column;
  flex-basis: 100%;
  margin: ${props => props.theme.space[1]}px;
  padding: 0;
  gap: 0;
`;

const TitleContainer = styled(Flex)`
  align-items: center;
  justify-content: space-between;
  min-height: ${p => p.theme.space[8]}px;
  padding-left: ${p => p.theme.space[3]}px;
  padding-right: ${p => p.theme.space[3]}px;
  gap: ${p => p.theme.space[2]}px;
`;

export const TitleText = styled(Text).attrs({
  as: 'h2',
  typography: 'h2',
})``;

const Divider = styled.div`
  height: 1px;
  flex-shrink: 0;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
`;

const InnerContainer = styled(Flex)`
  overflow: auto;
  flex-direction: column;
  padding: ${p => p.theme.space[3]}px;
`;

function UpgradeStatusChart(props: {
  data: GetBotInstanceMetricsResponse['upgrade_statuses'];
  onFilterSelected: (status: string) => void;
}) {
  const { data, onFilterSelected } = props;

  const theme = useTheme();

  const max = Math.max(
    1, // Never zero
    data?.up_to_date?.count ?? 0,
    data?.patch_available?.count ?? 0,
    data?.requires_upgrade?.count ?? 0,
    data?.unsupported?.count ?? 0
  );

  const total = Math.max(
    1, // Never zero
    (data?.up_to_date?.count ?? 0) +
      (data?.patch_available?.count ?? 0) +
      (data?.requires_upgrade?.count ?? 0) +
      (data?.unsupported?.count ?? 0)
  );

  const series = data
    ? [
        {
          name: 'Up to date',
          percent: (data.up_to_date?.count ?? 0) / max,
          count: data.up_to_date?.count ?? 0,
          label: `${data.up_to_date?.count ?? 0}\xa0(${formatPercent((data.up_to_date?.count ?? 0) / total)})`,
          color: theme.colors.interactive.solid.success.default,
          onClick: () =>
            data.up_to_date?.filter
              ? onFilterSelected(data.up_to_date?.filter)
              : undefined,
          tooltip:
            'Up-to-date instances are running the same version as the Teleport cluster.',
        },
        {
          name: 'Patch available',
          percent: (data.patch_available?.count ?? 0) / max,
          count: data.patch_available?.count ?? 0,
          label: `${data.patch_available?.count ?? 0}\xa0(${formatPercent((data.patch_available?.count ?? 0) / total)})`,
          color: theme.colors.interactive.solid.accent.default,
          onClick: () =>
            data.patch_available?.filter
              ? onFilterSelected(data.patch_available?.filter)
              : undefined,
          tooltip:
            'Instances with a patch available are running the same major version as the Teleport cluster.',
        },
        {
          name: 'Upgrade required',
          percent: (data.requires_upgrade?.count ?? 0) / max,
          count: data.requires_upgrade?.count ?? 0,
          label: `${data.requires_upgrade?.count ?? 0}\xa0(${formatPercent((data.requires_upgrade?.count ?? 0) / total)})`,
          color: theme.colors.interactive.solid.alert.default,
          onClick: () =>
            data.requires_upgrade?.filter
              ? onFilterSelected(data.requires_upgrade?.filter)
              : undefined,
          tooltip:
            'Instances requiring an upgrade are running the one major version behind the Teleport cluster.',
        },
        {
          name: 'Unsupported',
          percent: (data.unsupported?.count ?? 0) / max,
          count: data.unsupported?.count ?? 0,
          label: `${data.unsupported?.count ?? 0}\xa0(${formatPercent((data.unsupported?.count ?? 0) / total)})`,
          color: theme.colors.interactive.solid.danger.default,
          onClick: () =>
            data.unsupported?.filter
              ? onFilterSelected(data.unsupported?.filter)
              : undefined,
          tooltip:
            'Unsupported instances are running two or more major versions behind the Teleport cluster, or are running a newer version.',
        },
      ]
    : null;

  return (
    <UpgradeStatusContainer>
      <ChartTitleText>Version Compatibility</ChartTitleText>
      <BarsContainer>
        {series ? (
          series.map(s => (
            <SeriesContainer
              key={s.name}
              onClick={s.onClick}
              onKeyUp={event => {
                if (event.key === 'Enter') {
                  s.onClick();
                }
              }}
              role="button"
              tabIndex={0}
              aria-label={`${s.name}`}
            >
              <ChartLabelContainer>
                <ChartLabelText>{s.name}</ChartLabelText>
                <IconTooltip kind="info" position="top">
                  {s.tooltip}
                </IconTooltip>
              </ChartLabelContainer>
              <Bar percent={s.percent} label={s.label} color={s.color} />
            </SeriesContainer>
          ))
        ) : (
          <ChartNoDataContainer>No data available</ChartNoDataContainer>
        )}
      </BarsContainer>

      {series ? (
        <Alert kind="info" m={0}>
          Select a status above to view instances.
        </Alert>
      ) : undefined}

      {data?.updated_at ? (
        <HoverTooltip
          placement="top"
          tipContent={format(parseISO(data.updated_at), 'PP, p z')}
        >
          <ChartUpdatedAtText>
            Last updated {formatDistanceToNowStrict(parseISO(data.updated_at))}{' '}
            ago
          </ChartUpdatedAtText>
        </HoverTooltip>
      ) : undefined}
    </UpgradeStatusContainer>
  );
}

const UpgradeStatusContainer = styled(Flex)`
  flex-direction: column;
  padding: ${({ theme }) => theme.space[3]}px;
  border-radius: ${({ theme }) => theme.space[4]}px;
  gap: ${({ theme }) => theme.space[3]}px;
  background-color: ${({ theme }) => theme.colors.levels.elevated};
`;

const ChartTitleText = styled(H3)`
  text-align: center;
`;

const BarsContainer = styled(Flex)`
  flex-direction: column;
`;

const SeriesContainer = styled.div`
  padding: ${({ theme }) => theme.space[2]}px ${({ theme }) => theme.space[3]}px;
  border-radius: ${({ theme }) => theme.space[2]}px;

  cursor: pointer;

  &:hover {
    background-color: ${({ theme }) => theme.colors.levels.surface};
  }
  &:focus,
  &:active {
    outline: none;

    background-color: ${({ theme }) => theme.colors.levels.sunken};
  }
`;

const ChartLabelContainer = styled(Flex)`
  align-items: center;
  gap: ${({ theme }) => theme.space[2]}px;
`;

const ChartLabelText = styled(Text)`
  white-space: nowrap;
  font-size: ${({ theme }) => theme.fontSizes[1]}px;
  font-weight: ${({ theme }) => theme.fontWeights.medium};
`;

const ChartNoDataContainer = styled(Flex)`
  align-items: center;
  justify-content: center;
  padding: ${({ theme }) => theme.space[4]}px;
  color: ${({ theme }) => theme.colors.text.muted};
`;

const ChartUpdatedAtText = styled(Text)`
  font-size: ${({ theme }) => theme.fontSizes[1]}px;
  font-weight: ${({ theme }) => theme.fontWeights.medium};
  align-self: flex-end;
`;

function Bar(props: { percent: number; label: string; color: string }) {
  const { percent, label, color } = props;

  return (
    <BarContainer>
      <BarAmount $percent={percent} $color={color} />
      <BarLabel $percent={percent}>{label}</BarLabel>
    </BarContainer>
  );
}

const BarContainer = styled(Flex)`
  align-items: center;
  gap: ${({ theme }) => theme.space[2]}px;
`;

const BarAmount = styled.div<{ $percent: number; $color: string }>`
  flex-grow: ${({ $percent }) => $percent};
  background-color: ${({ $color }) => $color};
  height: ${({ theme }) => theme.space[3]}px;
  border-radius: ${({ theme }) => theme.space[1]}px;
  min-width: ${({ theme }) => theme.space[1]}px;

  transition: flex-grow 1000ms ease-in-out;
`;

const BarLabel = styled.div<{ $percent: number }>`
  flex-grow: ${({ $percent }) => 1 - $percent};

  transition: flex-grow 1000ms ease-in-out;
`;

function formatPercent(percent: number) {
  return `${(percent * 100).toFixed(0)}%`;
}
