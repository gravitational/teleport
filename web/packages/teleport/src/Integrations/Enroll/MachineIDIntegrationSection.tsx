/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { Box, H2 } from 'design';
import { P } from 'design/Text/Text';

import { BotTiles } from 'teleport/Bots/Add/AddBotsPicker';
import useTeleport from 'teleport/useTeleport';

export const MachineIDIntegrationSection = () => {
  const ctx = useTeleport();
  return (
    <>
      <Box mb={3}>
        <H2 mb={1}>Machine ID</H2>
        <P>
          Set up Teleport Machine ID to allow CI/CD workflows and other machines
          to access resources protected by Teleport.
        </P>
      </Box>
      <BotTiles hasCreateBotPermission={ctx.getFeatureFlags().addBots} />
    </>
  );
};
