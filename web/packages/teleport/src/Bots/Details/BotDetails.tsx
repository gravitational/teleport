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
import styled from 'styled-components';

import { Alert } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import ButtonIcon from 'design/ButtonIcon/ButtonIcon';
import Flex from 'design/Flex/Flex';
import { ArrowLeft } from 'design/Icon/Icons/ArrowLeft';
import { Indicator } from 'design/Indicator/Indicator';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide/InfoGuide';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import useTeleport from 'teleport/useTeleport';

import { useGetBot } from '../hooks';
import { InfoGuide } from '../InfoGuide';
import { Config } from './Config';
import { Panel } from './Panel';
import { Roles } from './Roles';
import { Traits } from './Traits';

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
          <ContentContainer>
            <ColumnContainer>
              <Panel title="Bot Details" />
              <Divider />
              <Config
                botName={data.name}
                maxSessionDurationSeconds={data.max_session_ttl?.seconds}
              />
              <PaddedDivider />
              <Roles roles={data.roles} />
              <PaddedDivider />
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

const Divider = styled.div`
  height: 1px;
  width: 100%;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
`;

const PaddedDivider = styled.div`
  height: 1px;
  background-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  margin-left: 16px;
  margin-right: 16px;
`;
