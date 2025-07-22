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

import format from 'date-fns/format';
import formatDistanceToNowStrict from 'date-fns/formatDistanceToNowStrict';
import parseISO from 'date-fns/parseISO';
import styled from 'styled-components';

import Flex from 'design/Flex/Flex';
import { Apple } from 'design/Icon/Icons/Apple';
import { ArrowFatLinesUp } from 'design/Icon/Icons/ArrowFatLinesUp';
import { Devices } from 'design/Icon/Icons/Devices';
import { Linux } from 'design/Icon/Icons/Linux';
import { Windows } from 'design/Icon/Icons/Windows';
import {
  DangerOutlined,
  SecondaryOutlined,
  WarningOutlined,
} from 'design/Label/Label';
import Text from 'design/Text/Text';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';

import { ClusterVersionDiff } from '../../useClusterVersion';
import { JoinMethodIcon } from './JoinMethodIcon';

export function Instance(props: {
  id: string;
  version?: string;
  versionDiff?: ClusterVersionDiff;
  hostname?: string;
  activeAt?: string;
  method?: string;
  os?: string;
}) {
  const { id, version, versionDiff, hostname, activeAt, method, os } = props;

  return (
    <Container>
      <TopRow>
        <Text fontWeight={'light'}>{id}</Text>
        {activeAt ? (
          <HoverTooltip
            placement="top"
            tipContent={format(parseISO(activeAt), 'PP, p z')}
          >
            <Text
              fontSize={0}
              fontWeight={'regular'}
            >{`${formatDistanceToNowStrict(parseISO(activeAt))} ago`}</Text>
          </HoverTooltip>
        ) : undefined}
      </TopRow>
      <BottomRow>
        <Flex gap={2}>
          <Version version={version} versionDiff={versionDiff} />

          {hostname ? (
            <HoverTooltip placement="top" tipContent={'Hostname'}>
              <SecondaryOutlined>
                <Text>{hostname}</Text>
              </SecondaryOutlined>
            </HoverTooltip>
          ) : undefined}
        </Flex>
        <Flex gap={2}>
          {method ? (
            <JoinMethodIcon method={method} size={'medium'} />
          ) : undefined}

          {os ? (
            <HoverTooltip placement="top" tipContent={os}>
              {os === 'darwin' ? (
                <Apple size={'medium'} />
              ) : os === 'windows' ? (
                <Windows size={'medium'} />
              ) : os === 'linux' ? (
                <Linux size={'medium'} />
              ) : (
                <Devices size={'medium'} />
              )}
            </HoverTooltip>
          ) : undefined}
        </Flex>
      </BottomRow>
    </Container>
  );
}

const Container = styled(Flex)`
  flex-direction: column;
  padding: ${props => props.theme.space[3]}px;
  padding-top: ${p => p.theme.space[2]}px;
  padding-bottom: ${p => p.theme.space[2]}px;
  background-color: ${p => p.theme.colors.levels.surface};
  gap: ${p => p.theme.space[1]}px;
`;

const TopRow = styled(Flex)`
  justify-content: space-between;
  align-items: center;
`;

const BottomRow = styled(Flex)`
  justify-content: space-between;
  align-items: flex-end;
`;

function Version(props: {
  version: string | undefined;
  versionDiff: ClusterVersionDiff | undefined;
}) {
  const { version, versionDiff } = props;

  const Wrapper = (() => {
    switch (versionDiff) {
      case 'n-2':
        return WarningOutlined;
      case 'n-':
        return DangerOutlined;
      default:
        return SecondaryOutlined;
    }
  })();

  const icon = versionDiff?.startsWith('n-') ? (
    <ArrowFatLinesUp size={'small'} />
  ) : undefined;

  const tooltip = (() => {
    switch (versionDiff) {
      case 'n-1':
        return 'Version is one major versions behind';
      case 'n-2':
        return 'Version is two major versions behind';
      case 'n-':
        return 'Version is more than two major versions behind';
      default:
        return 'Version is up to date';
    }
  })();

  return version ? (
    <HoverTooltip placement="top" tipContent={tooltip}>
      <Wrapper>
        <Flex gap={1}>
          {icon}
          <Text>v{version}</Text>
        </Flex>
      </Wrapper>
    </HoverTooltip>
  ) : undefined;
}
