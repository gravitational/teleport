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
import Flex from 'design/Flex/Flex';
import { ArrowLeft } from 'design/Icon/Icons/ArrowLeft';
import { Pencil } from 'design/Icon/Icons/Pencil';
import { Question } from 'design/Icon/Icons/Question';
import { Indicator } from 'design/Indicator/Indicator';
import { Outline } from 'design/Label/Label';
import Text from 'design/Text';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide/InfoGuide';
import { traitsPreset } from 'shared/components/TraitsEditor/TraitsEditor';
import { CopyButton } from 'shared/components/UnifiedResources/shared/CopyButton';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import useTeleport from 'teleport/useTeleport';

import { EditDialog } from '../Edit/EditDialog';
import { formatDuration } from '../formatDuration';
import { useGetBot } from '../hooks';
import { InfoGuide } from '../InfoGuide';
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
        <Box data-testid="loading" textAlign="center" m={10}>
          <Indicator />
        </Box>
      ) : undefined}

      {isError ? (
        <Alert kind="danger">Error: {error.message}</Alert>
      ) : undefined}

      {isSuccess && data === null && (
        <Alert kind="warning">Bot {params.botName} does not exist</Alert>
      )}

      {!hasReadPermission && (
        <Alert kind="info">
          <Flex gap={2}>
            You do not have permission to view this bot. Missing role
            permissions: <code>bots.read</code>
          </Flex>
        </Alert>
      )}

      {hasReadPermission && isSuccess && data ? (
        <Container>
          <ColumnContainer>
            <Panel
              title="Bot Details"
              action={{
                label: 'Edit',
                onClick: handleEdit,
                icon: <Pencil size={'medium'} />,
                disabled: !hasEditPermission,
              }}
            />
            <Divider />

            <Panel title="Metadata" isSubPanel>
              <Grid>
                <GridLabel>Botname</GridLabel>
                <Flex inline alignItems={'center'} gap={1} mr={0}>
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
            </Panel>

            <PaddedDivider />

            <Panel title="Roles" isSubPanel>
              {data.roles.length ? (
                <Flex>
                  {data.roles.toSorted().map(r => (
                    <Outline mr="1" key={r}>
                      {r}
                    </Outline>
                  ))}
                </Flex>
              ) : (
                'No roles assigned'
              )}
            </Panel>

            <PaddedDivider />

            <Panel title="Traits" isSubPanel>
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
                                <Outline mr="1" key={v}>
                                  {v}
                                </Outline>
                              ))
                            : 'no values'}
                        </div>
                      </React.Fragment>
                    ))}
                </Grid>
              ) : (
                'No traits set'
              )}
            </Panel>

            <Divider />

            <Panel title="Join Tokens">Coming soon</Panel>
          </ColumnContainer>
          <ColumnContainer>
            <Panel title="Active Instances">Coming soon</Panel>
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

const Container = styled(Flex).attrs({ gap: 3 })`
  flex-wrap: wrap;
`;

const ColumnContainer = styled(Flex)`
  flex: 1;
  flex-direction: column;
  min-width: 300px;
  background-color: ${p => p.theme.colors.levels.surface};
  border-radius: ${props => props.theme.space[1]}px;
`;

const Divider = styled.div`
  height: 1px;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
`;

const PaddedDivider = styled(Divider)`
  margin-left: ${props => props.theme.space[3]}px;
  margin-right: ${props => props.theme.space[3]}px;
`;

const Grid = styled.div`
  display: grid;
  grid-template-columns: repeat(2, auto);
`;

const GridLabel = styled(Text)`
  color: ${({ theme }) => theme.colors.text.muted};
  font-weight: ${({ theme }) => theme.fontWeights.regular};
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
