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
import { ReactElement } from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex/Flex';
import { ArrowFatLinesUp } from 'design/Icon/Icons/ArrowFatLinesUp';
import {
  DangerOutlined,
  SecondaryOutlined,
  WarningOutlined,
} from 'design/Label/Label';
import { ResourceIcon } from 'design/ResourceIcon';
import Text from 'design/Text/Text';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';

import {
  ClientCompatibility,
  useClusterVersion,
} from '../../useClusterVersion';
import { JoinMethodIcon } from './JoinMethodIcon';

export function Instance(props: {
  id: string;
  version?: string;
  hostname?: string;
  activeAt?: string;
  method?: string;
  os?: string;
}) {
  const { id, version, hostname, activeAt, method, os } = props;
  const { check } = useClusterVersion();
  const versionCheck = check(version);

  const hasHeartbeatData = !!version || !!hostname || !!method || !!os;

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
      {hasHeartbeatData ? (
        <BottomRow>
          <Flex gap={2}>
            <Version version={version} check={versionCheck} />

            {hostname ? (
              <HoverTooltip placement="top" tipContent={'Hostname'}>
                <SecondaryOutlined>
                  <Text fontSize={'12px'}>{hostname}</Text>
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
                <OsIconContainer>
                  {os === 'darwin' ? (
                    <ResourceIcon name={'apple'} width={'16px'} />
                  ) : os === 'windows' ? (
                    <ResourceIcon name={'windows'} width={'16px'} />
                  ) : os === 'linux' ? (
                    <ResourceIcon name={'linux'} width={'16px'} />
                  ) : (
                    <ResourceIcon name={'server'} width={'16px'} />
                  )}
                </OsIconContainer>
              </HoverTooltip>
            ) : undefined}
          </Flex>
        </BottomRow>
      ) : (
        <EmptyText>No heartbeat data</EmptyText>
      )}
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

const OsIconContainer = styled(Flex)`
  width: 20px; // Intentionally not a theme value
  height: 20px; // Intentionally not a theme value
  align-items: center;
  justify-content: center;
`;

const EmptyText = styled(Text)`
  color: ${p => p.theme.colors.text.muted};
`;

function Version(props: {
  version: string | undefined;
  check: ClientCompatibility;
}) {
  const { version, check } = props;

  let Wrapper = SecondaryOutlined;
  let icon: ReactElement | null = <ArrowFatLinesUp size={'small'} />;
  let tooltip = 'Version is up to date';
  if (check?.isCompatible) {
    switch (check.reason) {
      case 'match':
        icon = null;
        break;
      case 'upgrade-minor':
        tooltip = 'An upgrade is available';
        break;
      case 'upgrade-major':
        Wrapper = WarningOutlined;
        tooltip =
          'Version is one major version behind. Consider upgrading soon.';
        break;
    }
  } else {
    switch (check?.reason) {
      case 'too-old':
        Wrapper = DangerOutlined;
        tooltip =
          'Version is two or more major versions behind, and is no longer compatible.';
        break;
      case 'too-new':
        Wrapper = DangerOutlined;
        tooltip =
          'Version is two or more major versions ahead, and is not compatible.';
        break;
    }
  }

  return version ? (
    <HoverTooltip placement="top" tipContent={tooltip}>
      <Wrapper>
        <Flex gap={1}>
          {icon}
          <Text fontSize={'12px'}>v{version}</Text>
        </Flex>
      </Wrapper>
    </HoverTooltip>
  ) : undefined;
}
