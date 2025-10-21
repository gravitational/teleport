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

import React from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import Box from 'design/Box/Box';
import Flex from 'design/Flex/Flex';
import { SecondaryOutlined } from 'design/Label/Label';
import Text from 'design/Text';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';
import { IconTooltip } from 'design/Tooltip/IconTooltip';
import { CopyButton } from 'shared/components/CopyButton/CopyButton';

import { Panel } from 'teleport/Bots/Details/Panel';
import { formatDuration } from 'teleport/Bots/formatDuration';
import cfg from 'teleport/config';
import {
  BotInstanceKind,
  BotInstanceServiceHealthStatus,
  GetBotInstanceResponse,
  GetBotInstanceResponseJoinAttrs,
} from 'teleport/services/bot/types';

export function InfoTab(props: {
  data: GetBotInstanceResponse;
  onGoToServicesClick: () => void;
}) {
  const {
    data: { bot_instance },
    onGoToServicesClick,
  } = props;

  const { spec, status } = bot_instance ?? {};
  const { bot_name } = spec ?? {};
  const { latest_heartbeats, latest_authentications, service_health } =
    status ?? {};
  const latestHeartbeat = latest_heartbeats?.at(-1);
  const { kind, uptime, version, os, hostname } = latestHeartbeat ?? {};
  const latestAuthentication = latest_authentications?.at(-1);
  const { join_attrs } = latestAuthentication ?? {};
  const { meta } = join_attrs ?? {};
  const { join_method, join_token_name } = meta ?? {};

  const joinExtras = makeJoinExtras(join_attrs);

  const { kindLabel, kindTooltip } = makeKindInfo(kind) ?? {};

  const healthyCount =
    service_health?.filter(
      h =>
        h.status ===
        BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_HEALTHY
    ).length ?? 0;
  const totalCount = service_health?.length ?? 0;

  return (
    <Container>
      <Panel title="Summary" isSubPanel>
        <PanelContentContainer>
          <Grid>
            <GridLabel>Bot name</GridLabel>
            {bot_name ? (
              <Flex inline alignItems={'center'} gap={1} overflow={'hidden'}>
                <GridValue>
                  <StyledLink to={cfg.getBotDetailsRoute(bot_name)}>
                    {bot_name}
                  </StyledLink>
                </GridValue>
                <CopyButton value={bot_name} />
              </Flex>
            ) : (
              '-'
            )}
            <GridLabel>Up time</GridLabel>
            {uptime?.seconds
              ? formatDuration(
                  { seconds: uptime.seconds },
                  {
                    separator: ' ',
                  }
                )
              : '-'}
            <GridLabel>Kind</GridLabel>
            {kindLabel ? (
              <Flex inline alignItems={'center'} gap={1} overflow={'hidden'}>
                <GridValue>{kindLabel}</GridValue>
                <IconTooltip kind="info" position="top">
                  {kindTooltip}
                </IconTooltip>
              </Flex>
            ) : (
              '-'
            )}
            <GridLabel>Version</GridLabel>
            <GridValue>{version ? `v${version}` : '-'}</GridValue>
            <GridLabel>OS</GridLabel>
            <GridValue>{os || '-'}</GridValue>
            <GridLabel>Hostname</GridLabel>
            <GridValue>{hostname || '-'}</GridValue>
          </Grid>
        </PanelContentContainer>
      </Panel>

      <PaddedDivider />

      <Panel
        title="Health Status"
        isSubPanel
        action={{
          label: 'View Services',
          onClick: onGoToServicesClick,
        }}
      >
        <PanelContentContainer gap={3}>
          <div>
            <AccentCountText as={'span'}>{healthyCount}</AccentCountText> of{' '}
            {totalCount} services are healthy
          </div>

          <HealthLabelsContainer>
            {service_health
              ?.toSorted((a, b) =>
                (a.service?.name ?? '').localeCompare(b.service?.name ?? '')
              )
              .map(h =>
                h.service?.name ? (
                  <HoverTooltip
                    key={h.service.name}
                    placement="top"
                    tipContent={makeHealthTooltip(h.status)}
                  >
                    <SecondaryOutlined>
                      <Flex
                        alignItems={'center'}
                        gap={2}
                        padding={1}
                        paddingLeft={0}
                        paddingRight={2}
                      >
                        <HealthStatusDot $status={h.status} />
                        <HealthLabelText>{h.service.name}</HealthLabelText>
                      </Flex>
                    </SecondaryOutlined>
                  </HoverTooltip>
                ) : undefined
              )}
          </HealthLabelsContainer>
        </PanelContentContainer>
      </Panel>

      <PaddedDivider />

      <Panel title="Join Token" isSubPanel>
        <PanelContentContainer>
          <Grid>
            <GridLabel>Name</GridLabel>
            {join_token_name ? (
              <Flex inline alignItems={'center'} gap={1} overflow={'hidden'}>
                <GridValue>
                  <StyledLink to={cfg.getJoinTokensRoute()}>
                    {join_token_name}
                  </StyledLink>
                </GridValue>
                <CopyButton value={join_token_name} />
              </Flex>
            ) : (
              '-'
            )}
            <GridLabel>Method</GridLabel>
            <GridValue>{join_method || '-'}</GridValue>

            {joinExtras
              ? joinExtras.map(([label, value]) => (
                  <React.Fragment key={label}>
                    <GridLabel>{label}</GridLabel>
                    <GridValue>{value || '-'}</GridValue>
                  </React.Fragment>
                ))
              : undefined}
          </Grid>
        </PanelContentContainer>
      </Panel>
    </Container>
  );
}

const Container = styled.div`
  flex: 1;
  min-width: 0;
`;

const PanelContentContainer = styled(Flex)`
  flex-direction: column;
  padding: ${props => props.theme.space[3]}px;
  padding-top: 0;
  overflow: hidden;
`;

const Grid = styled(Box)`
  align-self: flex-start;
  display: grid;
  grid-template-columns: repeat(2, auto);
  gap: ${({ theme }) => theme.space[2]}px;
  overflow: hidden;
`;

const GridLabel = styled(Text)`
  color: ${({ theme }) => theme.colors.text.muted};
  font-weight: ${({ theme }) => theme.fontWeights.regular};
  padding-right: ${({ theme }) => theme.space[2]}px;
`;

const GridValue = styled(Text)`
  white-space: nowrap;
`;

const PaddedDivider = styled.div`
  height: 1px;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  flex-shrink: 0;
  margin-left: ${props => props.theme.space[3]}px;
  margin-right: ${props => props.theme.space[3]}px;
`;

const HealthLabelsContainer = styled(Flex)`
  flex-wrap: wrap;
  overflow: hidden;
  gap: ${props => props.theme.space[1]}px;
`;

const HealthLabelText = styled(Text).attrs({
  typography: 'body3',
})`
  white-space: nowrap;
`;

const HealthStatusDot = styled.div<{
  $status: BotInstanceServiceHealthStatus | undefined;
}>`
  width: ${props => props.theme.space[3] - props.theme.space[1]}px;
  height: ${props => props.theme.space[3] - props.theme.space[1]}px;
  border-radius: 999px;
  background-color: ${({ theme, $status }) =>
    $status ===
    BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_HEALTHY
      ? theme.colors.interactive.solid.success.default
      : $status ===
          BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY
        ? theme.colors.interactive.solid.danger.default
        : $status ===
            BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_UNSPECIFIED
          ? theme.colors.interactive.solid.alert.default
          : theme.colors.interactive.tonal.neutral[1]};
`;

const AccentCountText = styled(Text)`
  font-size: ${({ theme }) => theme.fontSizes[8]}px;
  font-weight: ${({ theme }) => theme.fontWeights.light}px;
`;

const StyledLink = styled(Link)`
  color: ${({ theme }) => theme.colors.interactive.solid.accent.default};
  background: none;
  text-decoration: underline;
  text-transform: none;

  &:hover {
    color: ${({ theme }) => theme.colors.interactive.solid.accent.hover};
  }

  &:active {
    color: ${({ theme }) => theme.colors.interactive.solid.accent.active};
  }
`;

function makeKindInfo(kind: BotInstanceKind | undefined) {
  if (kind === BotInstanceKind.BOT_KIND_TBOT) {
    return {
      kindLabel: 'tbot',
      kindTooltip: 'This instance is running using the tbot CLI.',
    };
  }
  if (kind === BotInstanceKind.BOT_KIND_TERRAFORM_PROVIDER) {
    return {
      kindLabel: 'Terraform',
      kindTooltip:
        'This instance is running using the Teleport Terraform Provider.',
    };
  }
  if (kind === BotInstanceKind.BOT_KIND_KUBERNETES_OPERATOR) {
    return {
      kindLabel: 'Kubernetes',
      kindTooltip:
        'This instance is running using the Teleport Kubernetes Operator.',
    };
  }
  if (kind === BotInstanceKind.BOT_KIND_TCTL) {
    return {
      kindLabel: 'tctl',
      kindTooltip: 'This instance is running inside tctl.',
    };
  }

  return undefined;
}

function makeHealthTooltip(status: BotInstanceServiceHealthStatus | undefined) {
  if (
    status ===
    BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_INITIALIZING
  ) {
    return 'Status: Initializing';
  }
  if (
    status === BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_HEALTHY
  ) {
    return 'Status: Healthy';
  }
  if (
    status ===
    BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY
  ) {
    return 'Status: Unhealthy';
  }
  return 'Status: Unspecified';
}

function makeJoinExtras(
  joinAttrs?: GetBotInstanceResponseJoinAttrs | null
): [string, string | undefined][] {
  if (joinAttrs?.azure) {
    return [
      ['Resource group', joinAttrs.azure.resource_group],
      ['Subscription', joinAttrs.azure.subscription],
    ];
  }
  if (joinAttrs?.azure_devops) {
    return [
      ['Repository ID', joinAttrs.azure_devops.pipeline?.repository_id],
      ['Subject', joinAttrs.azure_devops.pipeline?.sub],
    ];
  }
  if (joinAttrs?.bitbucket) {
    return [
      ['Repository UUID', joinAttrs.bitbucket.repository_uuid],
      ['Subject', joinAttrs.bitbucket.sub],
    ];
  }
  if (joinAttrs?.circleci) {
    return [
      ['Project ID', joinAttrs.circleci.project_id],
      ['Subject', joinAttrs.circleci.sub],
    ];
  }
  if (joinAttrs?.gcp) {
    return [['Service account', joinAttrs.gcp.service_account]];
  }
  if (joinAttrs?.github) {
    return [
      ['Repository', joinAttrs.github.repository],
      ['Subject', joinAttrs.github.sub],
    ];
  }
  if (joinAttrs?.gitlab) {
    return [
      ['Project path', joinAttrs.gitlab.project_path],
      ['Subject', joinAttrs.gitlab.sub],
    ];
  }
  if (joinAttrs?.iam) {
    return [
      ['Account', joinAttrs.iam.account],
      ['ARN', joinAttrs.iam.arn],
    ];
  }
  if (joinAttrs?.kubernetes) {
    return [['Subject', joinAttrs.kubernetes.subject]];
  }
  if (joinAttrs?.oracle) {
    return [
      ['Tenancy ID', joinAttrs.oracle.tenancy_id],
      ['Compartment ID', joinAttrs.oracle.compartment_id],
    ];
  }
  if (joinAttrs?.spacelift) {
    return [
      ['Space ID', joinAttrs.spacelift.space_id],
      ['Subject', joinAttrs.spacelift.sub],
    ];
  }
  if (joinAttrs?.terraform_cloud) {
    return [
      ['Workspace', joinAttrs.terraform_cloud.full_workspace],
      ['Subject', joinAttrs.terraform_cloud.sub],
    ];
  }
  if (joinAttrs?.tpm) {
    return [['Public key', joinAttrs.tpm.ek_pub_hash]];
  }
  return [];
}
