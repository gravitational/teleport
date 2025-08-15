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
import * as Icons from 'web/packages/design/src/Icon';

import { P3, SyncStamp, Text } from 'design';
import Box from 'design/Box';
import { CardTile } from 'design/CardTile';
import Flex from 'design/Flex';
import { H2, H3, P2 } from 'design/Text';

export function ConsoleCard({
  enrolled,
  filters,
  groups,
  lastUpdated,
  profiles,
  roles,
}: {
  filters?: string[];
  groups?: number;
  lastUpdated?: number;
  profiles?: number;
  roles?: number;
  enrolled: boolean;
}) {
  if (!enrolled) {
    return <EnrollCard />;
  }

  return (
    <EnrolledCard
      filters={filters}
      groups={groups}
      lastUpdated={lastUpdated}
      profiles={profiles}
      roles={roles}
    />
  );
}

function EnrolledCard({
  filters,
  groups,
  lastUpdated,
  profiles,
  roles,
}: {
  filters?: string[];
  groups?: number;
  lastUpdated?: number;
  profiles?: number;
  roles?: number;
}) {
  const updated = lastUpdated ? new Date(lastUpdated) : undefined;

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
              {profiles}
            </Text>
            <H3>Profiles</H3>
            <P3 color="text.muted" mb={1}>
              AWS Roles Anywhere Profiles are available on the Resources Page.
            </P3>
            <Box borderTop={1} borderColor="interactive.tonal.neutral.0">
              <Text
                color="text.muted"
                fontSize={2}
                fontWeight="300"
                mb={2}
                mt={1}
                css={{ fontStyle: 'italic' }}
              >
                All {groups} Groups are being synced
              </Text>
            </Box>
          </Box>
          <Box>
            <Text fontWeight="500" fontSize={9} mb={2} css={{ lineHeight: 1 }}>
              {roles}
            </Text>
            <H3>Roles</H3>
            <P3 color="text.muted" mb={1}>
              AWS IAM Roles are available as dropdown options for the respective
              Profiles.
            </P3>
            <Box borderTop={1} borderColor="interactive.tonal.neutral.0">
              <Text
                color="text.muted"
                fontSize={2}
                fontWeight="300"
                mb={2}
                mt={1}
                css={{ fontStyle: 'italic' }}
              >
                Filtered By: {filters.join(', ')}
              </Text>
            </Box>
          </Box>
        </Flex>
        <SyncStamp date={updated} />
      </Flex>
    </CardTile>
  );
}

function EnrollCard() {
  return (
    <CardTile width="100%" data-testid={`console-enroll`}>
      <Flex flexDirection="column" justifyContent="space-between" height="100%">
        <Box>
          <Flex alignItems="center">
            <H2>AWS Console and CLI Access</H2>
          </Flex>
          <P2 mb={2} color="text.slightlyMuted">
            Create new app resources to access your AWS account.
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
