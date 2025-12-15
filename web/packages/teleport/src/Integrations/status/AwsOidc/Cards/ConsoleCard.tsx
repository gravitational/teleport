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
import { Link as InternalLink } from 'react-router-dom';
import styled, { useTheme } from 'styled-components';

import { P3, SyncStamp, Text } from 'design';
import Box from 'design/Box';
import { CardTile } from 'design/CardTile';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import { H2, H3, P2 } from 'design/Text';

import cfg from 'teleport/config';
import {
  IntegrationKind,
  RolesAnywhereProfileSync,
} from 'teleport/services/integrations';

export function ConsoleCardEnrolled({
  stats,
}: {
  stats: RolesAnywhereProfileSync;
}) {
  const theme = useTheme();
  const { syncedProfiles, syncEndTime } = stats;
  const updated = syncEndTime ? new Date(syncEndTime) : undefined;

  return (
    <CardTile width="100%" data-testid={`console-enrolled`}>
      <Flex flexDirection="column" justifyContent="space-between" height="100%">
        <Box>
          <Flex alignItems="center" justifyContent="space-between">
            <H2>AWS Console and CLI Access</H2>
            <Chip
              backgroundColor="interactive.tonal.success.0"
              pr={3}
              pl={2}
              py={1}
            >
              <Icons.CircleCheck
                size="small"
                color="interactive.solid.success.default"
              />
              <Text
                color="interactive.solid.success.default"
                typography="body3"
              >
                Enabled
              </Text>
            </Chip>
          </Flex>
          <P2 mb={2} color="text.slightlyMuted">
            Sync AWS IAM Roles Anywhere Profiles with Teleport
          </P2>
        </Box>
        <Flex gap={3} my={3}>
          <Box>
            <Text fontWeight="500" fontSize={9} mb={2} css={{ lineHeight: 1 }}>
              {syncedProfiles || 0}
            </Text>
            <H3>Profiles</H3>
            {syncedProfiles > 0 ? (
              <P3 color="text.muted" mb={1}>
                {' '}
                AWS Roles Anywhere Profiles are available on the{' '}
                <InternalLink
                  to={cfg.getUnifiedResourcesRoute(cfg.proxyCluster)}
                  style={{ color: theme.colors.text.muted }}
                >
                  Resources Page.
                </InternalLink>
              </P3>
            ) : (
              <P3 color="text.muted" mb={1}>
                Edit the integration to sync profiles.
              </P3>
            )}
          </Box>
        </Flex>
        <SyncStamp date={updated} />
      </Flex>
    </CardTile>
  );
}

export function ConsoleCardEnroll() {
  return (
    <CardTile
      width="100%"
      data-testid={`console-enroll`}
      as={InternalLink}
      to={cfg.getIntegrationEnrollRoute(IntegrationKind.AwsRa)}
    >
      <Flex flexDirection="column" justifyContent="space-between" height="100%">
        <Box>
          <Flex alignItems="center">
            <H2>AWS Console and CLI Access</H2>
          </Flex>
          <P2 mb={2} color="text.slightlyMuted">
            Configure access to your AWS account.
          </P2>
        </Box>
        <Flex alignItems="center" gap={2}>
          <H3>Enable Access</H3>
          <Icons.ArrowForward />
        </Flex>
      </Flex>
    </CardTile>
  );
}

const Chip = styled(Flex).attrs({
  flexDirection: 'row',
  alignItems: 'center',
  gap: 1,
})`
  border-radius: 999px;
`;
