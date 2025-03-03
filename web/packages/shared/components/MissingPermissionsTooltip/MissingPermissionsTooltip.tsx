/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Box, Flex, Text } from 'design';

export const MissingPermissionsTooltip = ({
  missingPermissions,
  requiresAll = true,
}: {
  missingPermissions: string[];
  requiresAll?: boolean;
}) => {
  return (
    <Box>
      <Text mb={1}>You do not have all of the required permissions.</Text>
      <Box mb={1}>
        {requiresAll ? (
          <Text bold>Missing permissions:</Text>
        ) : (
          <Text bold>
            You must have at least one of these role permissions:
          </Text>
        )}
        <Flex gap={2}>
          {missingPermissions.map(perm => (
            <Text key={perm}>{perm}</Text>
          ))}
        </Flex>
      </Box>
    </Box>
  );
};
