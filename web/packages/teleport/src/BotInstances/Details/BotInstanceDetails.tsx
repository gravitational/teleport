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
import TextEditor from 'shared/components/TextEditor/TextEditor';
import { CopyButton } from 'shared/components/UnifiedResources/shared/CopyButton';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';

import { useGetBotInstance } from '../hooks';

const docsUrl =
  'https://goteleport.com/docs/enroll-resources/machine-id/introduction/#bot-instances';

export function BotInstanceDetails(props: {
  onDocsLinkClickedForTesting?: MouseEventHandler<HTMLAnchorElement>;
}) {
  const history = useHistory();
  const params = useParams<{
    botName: string;
    instanceId: string;
  }>();

  const { data, error, isSuccess, isError, isLoading } = useGetBotInstance(
    params,
    {
      staleTime: 30_000, // Keep data in the cache for 30 seconds
    }
  );

  const handleBackPress = useCallback(() => {
    history.goBack();
  }, [history]);

  return (
    <FeatureBox>
      <FeatureHeader justifyContent="space-between" gap={2}>
        <Flex gap={2}>
          <HoverTooltip placement="bottom" tipContent={'Go back'}>
            <ButtonIcon onClick={handleBackPress} aria-label="back">
              <ArrowLeft size="medium" />
            </ButtonIcon>
          </HoverTooltip>
          <FeatureHeaderTitle>Bot instance</FeatureHeaderTitle>
          {isSuccess && data.bot_instance?.spec?.instance_id ? (
            <InstanceId>
              <Flex inline alignItems={'center'} gap={1} mr={0}>
                <MonoText>
                  {data.bot_instance.spec.instance_id.substring(0, 7)}
                </MonoText>
                <CopyButton name={data.bot_instance.spec.instance_id} />
              </Flex>
            </InstanceId>
          ) : undefined}
        </Flex>

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
      </FeatureHeader>

      {isLoading ? (
        <Box data-testid="loading" textAlign="center" m={10}>
          <Indicator />
        </Box>
      ) : undefined}

      {isError ? (
        <Alert kind="danger">{`Error: ${error.message}`}</Alert>
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
    </FeatureBox>
  );
}

const MonoText = styled(Text)`
  font-family: ${({ theme }) => theme.fonts.mono};
`;

const InstanceId = styled.div`
  display: flex;
  align-items: center;
  padding-left: 8px;
  padding-right: 6px;
  height: 32px;
  border-radius: 16px;
  background-color: ${({ theme }) => theme.colors.interactive.tonal.neutral[0]};
`;

const YamlContaner = styled(Flex)`
  flex: 1;
  border-radius: 8px;
  background-color: ${({ theme }) => theme.colors.levels.elevated};
`;
