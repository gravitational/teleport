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

import { MouseEventHandler, useCallback } from 'react';
import { useHistory, useParams } from 'react-router';
import styled from 'styled-components';

import { Alert } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import { ButtonBorder } from 'design/Button/Button';
import ButtonIcon from 'design/ButtonIcon/ButtonIcon';
import Flex from 'design/Flex/Flex';
import { ArrowLeft } from 'design/Icon/Icons/ArrowLeft';
import { Indicator } from 'design/Indicator/Indicator';
import Text from 'design/Text';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import useTeleport from 'teleport/useTeleport';

import { useGetBot } from '../hooks';
import { Config } from './Config';
import { Panel } from './Panel';
import { Roles } from './Roles';
import { Traits } from './Traits';

const docsUrl =
  'https://goteleport.com/docs/machine-workload-identity/machine-id/introduction/#bots';

export function BotDetails(props: {
  onDocsLinkClickedForTesting?: MouseEventHandler<HTMLAnchorElement>;
}) {
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
      </FeatureHeader>

      {isLoading ? (
        <Box data-testid="loading" textAlign="center" m={10}>
          <Indicator />
        </Box>
      ) : undefined}

      {isError ? (
        <Alert kind="danger">{`Error: ${error.message}`}</Alert>
      ) : undefined}

      {data === null && (
        <Alert kind="warning">{`Bot ${params.name} does not exist`}</Alert>
      )}

      {!hasReadPermission && (
        <Alert kind="warning">
          You do not have permission to view this bot.
        </Alert>
      )}

      {hasReadPermission && isSuccess && data ? (
        <Container>
          <IntroContainer>
            <IntroText>
              Teleport Bots enable machines, such as CI/CD workflows, to
              securely authenticate with your Teleport cluster in order to
              connect to resources and configure the cluster itself. This is
              sometimes referred to as machine-to-machine access.
            </IntroText>

            <ButtonBorder
              size="medium"
              as="a"
              href={docsUrl}
              target="_blank"
              rel="noreferrer"
              onClick={props.onDocsLinkClickedForTesting}
            >
              View Documentation
            </ButtonBorder>
          </IntroContainer>

          <ContentContainer>
            <ColumnContainer>
              <Config
                botName={data.name}
                maxSessionDurationSeconds={data.max_session_ttl?.seconds}
              />
              <Divider />
              <Roles roles={data.roles} />
              <Divider />
              <Traits traits={data.traits} />
              <Divider />
              <Panel title="Join Tokens">Coming soon</Panel>
            </ColumnContainer>
            <ColumnContainer>
              <Panel title="Active Instances">Coming soon</Panel>
            </ColumnContainer>
          </ContentContainer>
        </Container>
      ) : undefined}
    </FeatureBox>
  );
}

const Container = styled(Flex)`
  flex-direction: column;
  gap: 24px;
`;

const ContentContainer = styled(Flex)`
  gap: 16px;
`;

const ColumnContainer = styled(Flex)`
  flex: 1;
  flex-direction: column;
  background-color: ${p => p.theme.colors.levels.surface};
  border-radius: 4px;
`;

const IntroContainer = styled(Flex)`
  flex-direction: column;
  align-items: flex-start;
  max-width: 800px;
  gap: 16px;
`;

const IntroText = styled(Text)``;

const Divider = styled.div`
  height: 1px;
  width: 100%;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
`;
