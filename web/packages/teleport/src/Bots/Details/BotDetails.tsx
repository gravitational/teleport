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

import { useCallback } from 'react';
import { useHistory, useParams } from 'react-router';
import styled, { useTheme } from 'styled-components';

import { Alert } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import ButtonIcon from 'design/ButtonIcon/ButtonIcon';
import Flex from 'design/Flex/Flex';
import { ArrowLeft } from 'design/Icon/Icons/ArrowLeft';
import { Question } from 'design/Icon/Icons/Question';
import { Indicator } from 'design/Indicator/Indicator';
import { Outline } from 'design/Label/Label';
import Text from 'design/Text';
import { fontWeights } from 'design/theme/typography';
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

import { formatDuration } from '../formatDuration';
import { useGetBot } from '../hooks';
import { InfoGuide } from '../InfoGuide';
import { Panel } from './Panel';

const botNameLabel = 'Bot name';
const maxSessionDurationLabel = 'Max session duration';

export function BotDetails() {
  const ctx = useTeleport();
  const history = useHistory();
  const params = useParams<{
    name: string;
  }>();
  const flags = ctx.getFeatureFlags();
  const hasReadPermission = flags.readBots;

  const { data, error, isSuccess, isError, isLoading } = useGetBot(params, {
    enabled: hasReadPermission,
    staleTime: 30_000, // Keep data in the cache for 30 seconds
  });

  const handleBackPress = useCallback(() => {
    history.goBack();
  }, [history]);

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
            <FeatureHeaderTitle>{data.name}</FeatureHeaderTitle>
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

      {data === null && (
        <Alert kind="warning">Bot {params.name} does not exist</Alert>
      )}

      {!hasReadPermission && (
        <Alert kind="warning">
          You do not have permission to view this bot.
        </Alert>
      )}

      {hasReadPermission && isSuccess && data ? (
        <Container>
          <ColumnContainer>
            <Panel title="Bot Details" />
            <Divider />

            <Panel title="Metadata" isSubPanel>
              <TransposedTable>
                <tbody>
                  <tr>
                    <th scope="row">{botNameLabel}</th>
                    <td>
                      <Flex inline alignItems={'center'} gap={1} mr={0}>
                        <MonoText>{data.name}</MonoText>
                        <CopyButton name={data.name} />
                      </Flex>
                    </td>
                  </tr>
                  <tr>
                    <th scope="row">{maxSessionDurationLabel}</th>
                    <td>
                      {data.max_session_ttl?.seconds
                        ? formatDuration(data.max_session_ttl)
                        : '-'}
                    </td>
                  </tr>
                </tbody>
              </TransposedTable>
            </Panel>

            <PaddedDivider />

            <Panel title="Roles" isSubPanel>
              <Flex>
                {data.roles.toSorted().map(r => (
                  <Outline mr="1" key={r}>
                    {r}
                  </Outline>
                ))}
              </Flex>
            </Panel>

            <PaddedDivider />

            <Panel title="Traits" isSubPanel>
              <TransposedTable>
                <tbody>
                  {data.traits
                    .toSorted((a, b) => a.name.localeCompare(b.name))
                    .map(r => (
                      <tr key={r.name}>
                        <th scope="row">
                          <Trait traitName={r.name} />
                        </th>
                        <td>
                          {r.values.length > 0
                            ? r.values.toSorted().map(v => (
                                <Outline mr="1" key={v}>
                                  {v}
                                </Outline>
                              ))
                            : 'no values'}
                        </td>
                      </tr>
                    ))}
                </tbody>
              </TransposedTable>
            </Panel>

            <Divider />

            <Panel title="Join Tokens">Coming soon</Panel>
          </ColumnContainer>
          <ColumnContainer>
            <Panel title="Active Instances">Coming soon</Panel>
          </ColumnContainer>
        </Container>
      ) : undefined}
    </FeatureBox>
  );
}

const Container = styled(Flex).attrs({ gap: 3 })``;

const ColumnContainer = styled(Flex)`
  flex: 1;
  flex-direction: column;
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

const TransposedTable = styled.table`
  th {
    text-align: start;
    padding-right: ${props => props.theme.space[3]}px;
    width: 1%; // Minimum width to fit content
    color: ${({ theme }) => theme.colors.text.muted};
    font-weight: ${fontWeights.regular};
  }
`;

const MonoText = styled(Text)`
  font-family: ${({ theme }) => theme.fonts.mono};
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
