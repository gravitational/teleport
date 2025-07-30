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

import React, { useState } from 'react';
import { useHistory, useParams } from 'react-router';
import styled, { useTheme } from 'styled-components';

import { Alert } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import { ButtonSecondary } from 'design/Button/Button';
import ButtonIcon from 'design/ButtonIcon/ButtonIcon';
import { CardTile } from 'design/CardTile/CardTile';
import Flex, { Stack } from 'design/Flex/Flex';
import { ArrowLeft } from 'design/Icon/Icons/ArrowLeft';
import { FingerprintSimple } from 'design/Icon/Icons/FingerprintSimple';
import { NewTab } from 'design/Icon/Icons/NewTab';
import { Pencil } from 'design/Icon/Icons/Pencil';
import { Question } from 'design/Icon/Icons/Question';
import { Indicator } from 'design/Indicator/Indicator';
import { SecondaryOutlined } from 'design/Label/Label';
import Text from 'design/Text';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide/InfoGuide';
import { CopyButton } from 'shared/components/UnifiedResources/shared/CopyButton';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import cfg from 'teleport/config';
import { isAdminActionRequiresMfaError } from 'teleport/services/api/api';
import { traitsPreset } from 'teleport/Users/UserAddEdit/TraitsEditor';
import useTeleport from 'teleport/useTeleport';

import { EditDialog } from '../Edit/EditDialog';
import { formatDuration } from '../formatDuration';
import { useGetBot, useListBotTokens } from '../hooks';
import { InfoGuide } from '../InfoGuide';
import { InstancesPanel } from './InstancesPanel';
import { JoinMethodIcon } from './JoinMethodIcon';
import { Panel } from './Panel';

export function BotDetails() {
  const ctx = useTeleport();
  const history = useHistory();
  const params = useParams<{
    botName: string;
  }>();
  const [isEditing, setEditing] = useState(false);

  const flags = ctx.getFeatureFlags();
  const hasReadPermission = flags.readBots;
  const hasEditPermission = flags.editBots;

  const { data, error, isSuccess, isError, isLoading } = useGetBot(params, {
    enabled: hasReadPermission,
    staleTime: 30_000, // Keep data in the cache for 30 seconds
  });

  const handleBackPress = () => {
    history.goBack();
  };

  const handleEdit = () => {
    setEditing(true);
  };

  const handleEditSuccess = () => {
    setEditing(false);
  };

  const handleViewAllTokensClicked = () => {
    history.push(cfg.getJoinTokensRoute());
  };

  return (
    <FeatureBox>
      <FeatureHeader gap={2} data-testid="page-header">
        <HoverTooltip placement="bottom" tipContent={'Go back'}>
          <ButtonIcon onClick={handleBackPress} aria-label="back">
            <ArrowLeft size="medium" />
          </ButtonIcon>
        </HoverTooltip>
        <Flex flex={1} gap={2} justifyContent="space-between">
          {isSuccess && data ? (
            <>
              <FeatureHeaderTitle>{data.name}</FeatureHeaderTitle>

              <EditButton onClick={handleEdit} disabled={!hasEditPermission}>
                <Pencil size="medium" /> Edit Bot
              </EditButton>
            </>
          ) : (
            <FeatureHeaderTitle>Bot details</FeatureHeaderTitle>
          )}
        </Flex>
        <InfoGuideButton config={{ guide: <InfoGuide /> }} />
      </FeatureHeader>

      {isLoading ? (
        <Box data-testid="loading-bot" textAlign="center" m={10}>
          <Indicator />
        </Box>
      ) : undefined}

      {isError ? (
        <Alert kind="danger" details={error.message}>
          Failed to fetch bot
        </Alert>
      ) : undefined}

      {isSuccess && data === null ? (
        <Alert kind="warning">Bot {params.botName} does not exist</Alert>
      ) : undefined}

      {!hasReadPermission ? (
        <Alert kind="info">
          You do not have permission to view this bot. Missing role permissions:{' '}
          <code>bots.read</code>
        </Alert>
      ) : undefined}

      {hasReadPermission && isSuccess && data ? (
        <Container>
          <ColumnContainer>
            <Panel
              title="Bot Details"
              action={{
                label: 'Edit',
                onClick: handleEdit,
                iconLeft: <Pencil size={'medium'} />,
                disabled: !hasEditPermission,
              }}
            />
            <Divider />

            <Panel title="Metadata" isSubPanel>
              <PanelContentContainer>
                <Grid>
                  <GridLabel>Bot name</GridLabel>
                  <Flex inline alignItems={'center'} gap={1}>
                    <MonoText>{data.name}</MonoText>
                    <CopyButton name={data.name} />
                  </Flex>
                  <GridLabel>Max session duration</GridLabel>
                  {data.max_session_ttl
                    ? formatDuration(data.max_session_ttl, {
                        separator: ' ',
                      })
                    : '-'}
                </Grid>
              </PanelContentContainer>
            </Panel>

            <PaddedDivider />

            <Panel title="Roles" isSubPanel>
              <PanelContentContainer>
                {data.roles.length ? (
                  <RolesContainer>
                    {data.roles.toSorted().map(r => (
                      <SecondaryOutlined mr="1" key={r}>
                        {r}
                      </SecondaryOutlined>
                    ))}
                  </RolesContainer>
                ) : (
                  'No roles assigned'
                )}
              </PanelContentContainer>
            </Panel>

            <PaddedDivider />

            <Panel title="Traits" isSubPanel>
              <PanelContentContainer>
                {data.traits.length ? (
                  <Grid>
                    {data.traits
                      .toSorted((a, b) => a.name.localeCompare(b.name))
                      .map(r => (
                        <React.Fragment key={r.name}>
                          <GridLabel>
                            <Trait traitName={r.name} />
                          </GridLabel>
                          <div>
                            {r.values.length > 0
                              ? r.values.toSorted().map(v => (
                                  <SecondaryOutlined mr="1" key={v}>
                                    {v}
                                  </SecondaryOutlined>
                                ))
                              : 'no values'}
                          </div>
                        </React.Fragment>
                      ))}
                  </Grid>
                ) : (
                  'No traits set'
                )}
              </PanelContentContainer>
            </Panel>

            <Divider />

            <JoinTokens
              botName={data.name}
              onViewAllClicked={handleViewAllTokensClicked}
            />
          </ColumnContainer>
          <ColumnContainer maxWidth={400}>
            <InstancesPanel botName={params.botName} />
          </ColumnContainer>

          {isEditing ? (
            <EditDialog
              botName={data.name}
              onCancel={() => setEditing(false)}
              onSuccess={handleEditSuccess}
            />
          ) : undefined}
        </Container>
      ) : undefined}
    </FeatureBox>
  );
}

const Container = styled(Flex).attrs({ gap: 2 })`
  flex: 1;
  overflow: auto;
`;

const ColumnContainer = styled(CardTile)`
  flex-direction: column;
  overflow: auto;
  padding: 0;
  gap: 0;
  margin: ${props => props.theme.space[1]}px;
`;

const PanelContentContainer = styled(Flex)`
  flex-direction: column;
  padding: ${props => props.theme.space[3]}px;
  padding-top: 0;
`;

const RolesContainer = styled.div``;

const Divider = styled.div`
  height: 1px;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  flex-shrink: 0;
`;

const PaddedDivider = styled(Divider)`
  margin-left: ${props => props.theme.space[3]}px;
  margin-right: ${props => props.theme.space[3]}px;
`;

const Grid = styled(Box)`
  align-self: flex-start;
  display: grid;
  grid-template-columns: repeat(2, auto);
  gap: ${({ theme }) => theme.space[2]}px;
`;

const GridLabel = styled(Text)`
  color: ${({ theme }) => theme.colors.text.muted};
  font-weight: ${({ theme }) => theme.fontWeights.regular};
  padding-right: ${({ theme }) => theme.space[2]}px;
`;

const MonoText = styled(Text)`
  font-family: ${({ theme }) => theme.fonts.mono};
`;

const EditButton = styled(ButtonSecondary)`
  gap: ${props => props.theme.space[2]}px;
`;

const traitDescriptions: { [key in (typeof traitsPreset)[number]]: string } = {
  aws_role_arns: 'List of allowed AWS role ARNS',
  azure_identities: 'List of Azure identities',
  db_names: 'List of allowed database names',
  db_roles: 'List of allowed database roles',
  db_users: 'List of allowed database users',
  gcp_service_accounts: 'List of GCP service accounts',
  kubernetes_groups: 'List of allowed Kubernetes groups',
  kubernetes_users: 'List of allowed Kubernetes users',
  logins: 'List of allowed logins',
  windows_logins: 'List of allowed Windows logins',
  host_user_gid: 'The group ID to use for auto-host-users',
  host_user_uid: 'The user ID to use for auto-host-users',
  github_orgs: 'List of allowed GitHub organizations for git command proxy',
};

function Trait(props: { traitName: string }) {
  const theme = useTheme();

  const description = traitDescriptions[props.traitName];

  const help = (
    <Question
      size={'small'}
      color={theme.colors.interactive.tonal.neutral[3]}
    />
  );

  return description ? (
    <Flex gap={1}>
      {props.traitName}
      <HoverTooltip placement="top" tipContent={description}>
        {help}
      </HoverTooltip>
    </Flex>
  ) : (
    props.traitName
  );
}

function JoinTokens(props: { botName: string; onViewAllClicked: () => void }) {
  const { botName, onViewAllClicked } = props;

  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const hasListPermission = flags.listTokens;

  const [skipAuthnRetry, setSkipAuthnRetry] = useState(true);

  const { data, error, isSuccess, isError, isLoading } = useListBotTokens(
    { botName, skipAuthnRetry },
    {
      enabled: hasListPermission,
      staleTime: 30_000, // Keep data in the cache for 30 seconds
    }
  );

  const requiresMfa = isError && isAdminActionRequiresMfaError(error);

  const handleVerifyClick = () => {
    setSkipAuthnRetry(false);
  };

  return (
    <Panel
      title="Join Tokens"
      isSubPanel
      action={{
        label: 'View All',
        onClick: onViewAllClicked,
        iconRight: <NewTab size={'medium'} />,
        disabled: !hasListPermission,
      }}
    >
      <PanelContentContainer>
        {isLoading ? (
          <Box data-testid="loading-tokens" textAlign="center" m={10}>
            <Indicator />
          </Box>
        ) : undefined}

        {!hasListPermission ? (
          <Alert kind="info">
            You do not have permission to view join tokens. Missing role
            permissions: <code>tokens.list</code>
          </Alert>
        ) : undefined}

        {requiresMfa ? (
          <MfaContainer>
            <MfaText fontWeight={'regular'}>
              Multi-factor authentication is required to view join tokens
            </MfaText>
            <MfaVerifyButton onClick={handleVerifyClick}>
              <FingerprintSimple size="medium" /> Authenticate
            </MfaVerifyButton>
          </MfaContainer>
        ) : undefined}

        {isError && !requiresMfa ? (
          <Alert kind="danger" details={error.message}>
            Failed to fetch join tokens
          </Alert>
        ) : undefined}

        {isSuccess ? (
          <>
            {data.items.length ? (
              <Flex gap={1} flexWrap={'wrap'}>
                {data.items
                  .toSorted((a, b) => a.safeName.localeCompare(b.safeName))
                  .map(t => {
                    return (
                      <SecondaryOutlined key={t.id}>
                        <HoverTooltip placement="top" tipContent={t.method}>
                          <Flex alignItems={'center'} gap={1}>
                            <JoinMethodIcon
                              method={t.method}
                              size={'small'}
                              includeTooltip={false}
                            />
                            {t.safeName}
                          </Flex>
                        </HoverTooltip>
                      </SecondaryOutlined>
                    );
                  })}
              </Flex>
            ) : (
              'No join tokens'
            )}
          </>
        ) : undefined}
      </PanelContentContainer>
    </Panel>
  );
}

const MfaContainer = styled(Stack)`
  align-items: center;
  gap: ${props => props.theme.space[2]}px;
  background-color: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  padding: ${props => props.theme.space[4]}px;
  border: ${props => props.theme.borders[1]}
    ${props => props.theme.colors.interactive.tonal.neutral[0]};
  border-radius: ${props => props.theme.radii[2]}px;
`;

const MfaText = styled(Text)`
  max-width: 216px;
  text-align: center;
`;

const MfaVerifyButton = styled(ButtonSecondary)`
  gap: ${props => props.theme.space[2]}px;
`;
