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

import { format } from 'date-fns/format';
import { formatDistanceToNowStrict } from 'date-fns/formatDistanceToNowStrict';
import { parseISO } from 'date-fns/parseISO';
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

import { useClusterVersion } from '../../useClusterVersion';
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

  const hasHeartbeatData = !!version || !!hostname || !!method || !!os;

  return (
    <Container>
      <TopRow>
        <IdText typography="body2">{id}</IdText>
        {activeAt ? (
          <HoverTooltip
            placement="top"
            tipContent={format(parseISO(activeAt), 'PP, p z')}
          >
            <TimeText>{`${formatDistanceToNowStrict(parseISO(activeAt))} ago`}</TimeText>
          </HoverTooltip>
        ) : undefined}
      </TopRow>
      {hasHeartbeatData ? (
        <BottomRow>
          <Flex gap={2} flex={1} overflow={'hidden'} alignItems={'flex-end'}>
            <Version version={version} />

            {hostname ? (
              <HoverTooltip
                placement="top"
                tipContent={`Hostname: ${hostname}`}
              >
                <SecondaryOutlined borderRadius={2}>
                  <HostnameText>{hostname}</HostnameText>
                </SecondaryOutlined>
              </HoverTooltip>
            ) : undefined}
          </Flex>
          <Flex gap={2}>
            {method ? (
              <JoinMethodIcon method={method} size={'large'} />
            ) : undefined}

            {os ? (
              <HoverTooltip placement="top" tipContent={os}>
                {os === 'darwin' ? (
                  <ResourceIcon name={'apple'} size={'large'} />
                ) : os === 'windows' ? (
                  <ResourceIcon name={'windows'} size={'large'} />
                ) : os === 'linux' ? (
                  <ResourceIcon name={'linux'} size={'large'} />
                ) : (
                  <ResourceIcon name={'server'} size={'large'} />
                )}
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
  overflow: hidden;
  gap: ${p => p.theme.space[2]}px;
`;

const BottomRow = styled(Flex)`
  justify-content: space-between;
  align-items: flex-end;
  gap: ${p => p.theme.space[2]}px;
  overflow: hidden;
`;

const EmptyText = styled(Text)`
  color: ${p => p.theme.colors.text.muted};
`;

const TimeText = styled(Text).attrs({
  typography: 'body4',
})`
  white-space: nowrap;
`;

const IdText = styled(Text)`
  flex: 1;
  white-space: nowrap;
`;

const HostnameText = styled(Text).attrs({
  typography: 'body3',
})`
  white-space: nowrap;
`;

function Version(props: { version: string | undefined }) {
  const { version } = props;
  const { checkCompatibility } = useClusterVersion();
  const versionCompatibility = checkCompatibility(version);

  let Wrapper = SecondaryOutlined;
  let icon: ReactElement | null = <ArrowFatLinesUp size={'small'} />;
  let tooltip = 'Version is up to date';
  if (versionCompatibility?.isCompatible) {
    switch (versionCompatibility.reason) {
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
    switch (versionCompatibility?.reason) {
      case 'too-old':
        Wrapper = DangerOutlined;
        tooltip =
          'Version is two or more major versions behind, and is no longer compatible.';
        break;
      case 'too-new':
        Wrapper = DangerOutlined;
        tooltip =
          'Version is one or more major versions ahead, and is not compatible.';
        break;
    }
  }

  return version ? (
    <VersionContainer>
      <HoverTooltip placement="top" tipContent={tooltip}>
        <Wrapper borderRadius={2}>
          <Flex gap={1}>
            {icon}
            <Text typography="body3">v{version}</Text>
          </Flex>
        </Wrapper>
      </HoverTooltip>
    </VersionContainer>
  ) : undefined;
}

const VersionContainer = styled.div`
  flex-shrink: 0;
`;
