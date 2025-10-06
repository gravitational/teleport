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

import styled from 'styled-components';

import { Alert } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import ButtonIcon from 'design/ButtonIcon/ButtonIcon';
import Flex from 'design/Flex/Flex';
import { Cross } from 'design/Icon/Icons/Cross';
import { Indicator } from 'design/Indicator/Indicator';
import Text from 'design/Text/Text';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';
import TextEditor from 'shared/components/TextEditor/TextEditor';

import useTeleport from 'teleport/useTeleport';

import { useGetBotInstance } from '../hooks';

export function BotInstanceDetails(props: {
  botName: string;
  instanceId: string;
  onClose: () => void;
}) {
  const { botName, instanceId, onClose } = props;

  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const hasReadPermission = flags.readBotInstances;

  const { data, error, isSuccess, isError, isLoading } = useGetBotInstance(
    {
      botName,
      instanceId,
    },
    {
      enabled: hasReadPermission,
      staleTime: 30_000, // Keep data in the cache for 30 seconds
    }
  );

  return (
    <Container>
      <TitleContainer>
        <Flex gap={2} alignItems={'center'}>
          <HoverTooltip placement="top" tipContent={'Close'}>
            <ButtonIcon onClick={() => onClose()} aria-label="close">
              <Cross size="medium" />
            </ButtonIcon>
          </HoverTooltip>
          <TitleText>Resource YAML</TitleText>
        </Flex>
      </TitleContainer>
      <Divider />
      <ContentContainer>
        {isLoading ? (
          <Box data-testid="loading" textAlign="center" m={10}>
            <Indicator />
          </Box>
        ) : undefined}

        {isError ? (
          <Alert m={3} kind="danger">
            {error.message}
          </Alert>
        ) : undefined}

        {!hasReadPermission ? (
          <Alert kind="info" m={3}>
            You do not have permission to read Bot instances. Missing role
            permissions: <code>bot_instance.read</code>
          </Alert>
        ) : undefined}

        {isSuccess && data.yaml ? (
          <YamlContaner>
            <TextEditor
              bg="levels.elevated"
              data={[
                {
                  content: data.yaml,
                  type: 'yaml',
                },
              ]}
              readOnly={true}
            />
          </YamlContaner>
        ) : undefined}
      </ContentContainer>
    </Container>
  );
}

const Container = styled.section`
  display: flex;
  flex-direction: column;
  flex: 1;
  border-left-color: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  border-left-width: 1px;
  border-left-style: solid;
`;

const TitleContainer = styled(Flex)`
  align-items: center;
  justify-content: space-between;
  height: ${p => p.theme.space[8]}px;
  padding-left: ${p => p.theme.space[3]}px;
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

const ContentContainer = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
`;

const YamlContaner = styled(Flex)`
  flex: 1;
  border-radius: ${props => props.theme.space[2]}px;
  background-color: ${({ theme }) => theme.colors.levels.elevated};
`;
