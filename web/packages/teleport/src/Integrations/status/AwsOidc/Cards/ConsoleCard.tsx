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

import * as Icons from 'web/packages/design/src/Icon';

import Box from 'design/Box';
import { CardTile } from 'design/CardTile';
import Flex from 'design/Flex';
import { H2, H3, P2 } from 'design/Text';

export function ConsoleCard() {
  return <EnrollCard />;
}

function EnrollCard() {
  return (
    <CardTile width="100%" data-testid={`console-enroll`}>
      <Flex flexDirection="column" justifyContent="space-between" height="100%">
        <Box>
          <Flex alignItems="center">
            <H2>AWS Console and CLI Access</H2>
          </Flex>
          <P2 mb={2}>
            {/*todo (michellescripts) updated copy from design*/}
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
