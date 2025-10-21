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

import { format } from 'date-fns/format';
import { formatDistanceToNowStrict } from 'date-fns/formatDistanceToNowStrict';
import styled from 'styled-components';

import Flex from 'design/Flex';
import { SecondaryOutlined } from 'design/Label/Label';
import Text from 'design/Text';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';

import {
  BotInstanceServiceHealthStatus,
  GetBotInstanceResponse,
} from 'teleport/services/bot/types';

export function HealthTab(props: { data: GetBotInstanceResponse }) {
  const { data } = props;
  const { bot_instance } = data ?? {};
  const { status } = bot_instance ?? {};
  const { service_health } = status ?? {};

  return (
    <Container>
      {service_health?.length ? (
        service_health
          ?.toSorted((a, b) =>
            (a.service?.name ?? '').localeCompare(b.service?.name ?? '')
          )
          .map(h =>
            h.service?.name ? (
              <ItemContainer key={h.service.name} data-testid={h.service.name}>
                <Flex
                  gap={3}
                  alignItems={'flex-start'}
                  justifyContent={'space-between'}
                >
                  <Flex flexDirection={'column'} overflow={'hidden'}>
                    <TitleText>{h.service.name}</TitleText>
                    {h.service.type ? (
                      <Text typography="body3">Type: {h.service.type}</Text>
                    ) : undefined}
                  </Flex>

                  <Flex
                    flexDirection={'column'}
                    alignItems={'flex-end'}
                    gap={1}
                  >
                    {h.updated_at?.seconds ? (
                      <HoverTooltip
                        placement="top"
                        tipContent={format(
                          new Date(h.updated_at.seconds * 1000),
                          'PP, p z'
                        )}
                      >
                        <TimeText>{`Reported ${formatDistanceToNowStrict(new Date(h.updated_at.seconds * 1000))} ago`}</TimeText>
                      </HoverTooltip>
                    ) : undefined}
                    <SecondaryOutlined>
                      <Flex
                        alignItems={'center'}
                        gap={2}
                        padding={1}
                        paddingLeft={2}
                        paddingRight={0}
                      >
                        <Text typography="body3">
                          {makeHealthLabel(h.status)}
                        </Text>
                        <HealthStatusDot $status={h.status} />
                      </Flex>
                    </SecondaryOutlined>
                  </Flex>
                </Flex>

                {h.reason ? (
                  <ReasonContainer $status={h.status}>
                    {h.reason}
                  </ReasonContainer>
                ) : undefined}
              </ItemContainer>
            ) : undefined
          )
      ) : (
        <EmptyText>No reported services</EmptyText>
      )}
    </Container>
  );
}

const Container = styled(Flex)`
  flex-direction: column;
  flex: 1;
  min-width: 0;
  padding: ${({ theme }) => theme.space[3]}px;
  gap: ${({ theme }) => theme.space[3]}px;
  overflow: auto;
`;

const ItemContainer = styled(Flex)`
  flex-direction: column;
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  border-radius: ${({ theme }) => theme.space[1]}px;
  padding: ${({ theme }) => theme.space[3]}px;
  gap: ${({ theme }) => theme.space[3]}px;
`;

const HealthStatusDot = styled.div<{
  $status: BotInstanceServiceHealthStatus | undefined;
}>`
  width: ${({ theme }) => theme.space[3] - theme.space[1]}px;
  height: ${({ theme }) => theme.space[3] - theme.space[1]}px;
  border-radius: 999px;
  background-color: ${({ theme, $status }) =>
    $status ===
    BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_HEALTHY
      ? theme.colors.interactive.solid.success.default
      : $status ===
          BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY
        ? theme.colors.interactive.solid.danger.default
        : $status ===
            BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_INITIALIZING
          ? theme.colors.interactive.tonal.neutral[1]
          : theme.colors.interactive.solid.alert.default};
`;

const ReasonContainer = styled.div<{
  $status: BotInstanceServiceHealthStatus | undefined;
}>`
  border-width: 0;
  border-left-width: ${({ theme }) => theme.space[1]}px;
  border-style: solid;
  border-color: ${({ theme, $status }) =>
    $status ===
    BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_HEALTHY
      ? theme.colors.interactive.solid.success.default
      : $status ===
          BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY
        ? theme.colors.interactive.solid.danger.default
        : $status ===
            BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_INITIALIZING
          ? theme.colors.interactive.tonal.neutral[1]
          : theme.colors.interactive.solid.alert.default};
  padding: 0 ${({ theme }) => theme.space[2]}px;
`;

const TitleText = styled(Text).attrs({
  typography: 'body2',
})`
  white-space: nowrap;
  font-weight: ${({ theme }) => theme.fontWeights.medium};
`;

const EmptyText = styled(Text)`
  color: ${p => p.theme.colors.text.muted};
`;

const TimeText = styled(Text).attrs({
  typography: 'body4',
})`
  white-space: nowrap;
`;

function makeHealthLabel(status: BotInstanceServiceHealthStatus | undefined) {
  if (
    status ===
    BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_INITIALIZING
  ) {
    return 'Initializing';
  }
  if (
    status === BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_HEALTHY
  ) {
    return 'Healthy';
  }
  if (
    status ===
    BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY
  ) {
    return 'Unhealthy';
  }
  return 'Unknown';
}
